package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func referenceGmpaySign(t *testing.T, params map[string]string, secret string) string {
	t.Helper()
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "signature" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	payload := ""
	for i, k := range keys {
		if i > 0 {
			payload += "&"
		}
		payload += k + "=" + params[k]
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestGmpaySign(t *testing.T) {
	params := map[string]string{
		"pid":        "100000000001",
		"order_id":   "invoice-1001",
		"currency":   "USD",
		"amount":     "12.50",
		"notify_url": "https://merchant.example/notify",
	}
	secret := "top-secret-key"

	got := gmpaySign(params, secret)
	want := referenceGmpaySign(t, params, secret)
	assert.Equal(t, want, got)
	assert.Len(t, got, 64, "HMAC-SHA256 hex output must be 64 characters")
}

func TestGmpaySignExcludesSignatureAndEmptyValues(t *testing.T) {
	withExtras := map[string]string{
		"pid":       "100000000001",
		"order_id":  "invoice-1001",
		"signature": "should-be-ignored",
		"unset":     "",
	}
	withoutExtras := map[string]string{
		"pid":      "100000000001",
		"order_id": "invoice-1001",
	}
	secret := "top-secret-key"

	assert.Equal(t, gmpaySign(withoutExtras, secret), gmpaySign(withExtras, secret))
}

func TestGmpayVerify(t *testing.T) {
	secret := "webhook-secret"
	notification := map[string]string{
		"pid":                  "100000000001",
		"order_id":             "invoice-1001",
		"amount":               "12.50",
		"actual_amount":        "12.50",
		"receive_address":      "0xabc123",
		"token":                "USDT",
		"block_transaction_id": "0xdeadbeef",
		"status":               "2",
	}
	notification["signature"] = gmpaySign(notification, secret)

	assert.True(t, GmpayVerify(notification, secret))

	t.Run("rejects a tampered field", func(t *testing.T) {
		tampered := map[string]string{}
		for k, v := range notification {
			tampered[k] = v
		}
		tampered["amount"] = "999.99"
		assert.False(t, GmpayVerify(tampered, secret))
	})

	t.Run("rejects the wrong secret", func(t *testing.T) {
		assert.False(t, GmpayVerify(notification, "wrong-secret"))
	})

	t.Run("rejects a missing signature", func(t *testing.T) {
		missing := map[string]string{"pid": "1", "order_id": "2"}
		assert.False(t, GmpayVerify(missing, secret))
	})
}

func setupGmpayServiceTest(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	previousDomain, previousPid, previousSecret := setting.GmpayDomain, setting.GmpayPid, setting.GmpaySecret
	setting.GmpayDomain = server.URL
	setting.GmpayPid = "100000000001"
	setting.GmpaySecret = "top-secret-key"
	t.Cleanup(func() {
		setting.GmpayDomain, setting.GmpayPid, setting.GmpaySecret = previousDomain, previousPid, previousSecret
	})
}

func TestCreateGmpayTransaction(t *testing.T) {
	setupGmpayServiceTest(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/payments/gmpay/v1/order/create-transaction", r.URL.Path)

		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "100000000001", body["pid"])
		assert.Equal(t, "invoice-1001", body["order_id"])
		assert.NotEmpty(t, body["signature"])
		assert.True(t, GmpayVerify(body, setting.GmpaySecret), "request must be verifiable with the configured secret")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status_code": 0,
			"message":     "ok",
			"data": map[string]any{
				"trade_id":      "12345678901234567890",
				"order_id":      body["order_id"],
				"amount":        body["amount"],
				"currency":      body["currency"],
				"status":        1,
				"status_detail": "pending",
				"payment_url":   "https://pay.example.com/checkout/abc",
			},
			"request_id": "req-1",
		})
	})

	order, err := CreateGmpayTransaction(context.Background(), "invoice-1001", "USD", "12.50", "https://merchant.example/notify")
	require.NoError(t, err)
	assert.Equal(t, "invoice-1001", order.OrderId)
	assert.Equal(t, "https://pay.example.com/checkout/abc", order.PaymentUrl)
	assert.Equal(t, GmpayStatusWaitingForPayment, order.Status)
}

func TestCreateGmpayTransactionSurfacesGatewayError(t *testing.T) {
	setupGmpayServiceTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status_code": 10009,
			"message":     "invalid parameters",
			"request_id":  "req-2",
		})
	})

	_, err := CreateGmpayTransaction(context.Background(), "invoice-1001", "USD", "12.50", "https://merchant.example/notify")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGmpayRequestFailed)
}

func TestQueryGmpayTransaction(t *testing.T) {
	setupGmpayServiceTest(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "invoice-1001", r.URL.Query().Get("order_id"))

		params := map[string]string{
			"pid":       r.URL.Query().Get("pid"),
			"order_id":  r.URL.Query().Get("order_id"),
			"signature": r.URL.Query().Get("signature"),
		}
		assert.True(t, GmpayVerify(params, setting.GmpaySecret))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status_code": 0,
			"data": map[string]any{
				"trade_id": "12345678901234567890",
				"order_id": "invoice-1001",
				"status":   GmpayStatusPaid,
			},
		})
	})

	order, err := QueryGmpayTransaction(context.Background(), "invoice-1001")
	require.NoError(t, err)
	assert.Equal(t, GmpayStatusPaid, order.Status)
}

func TestGmpayRequestNotConfigured(t *testing.T) {
	previousDomain, previousPid, previousSecret := setting.GmpayDomain, setting.GmpayPid, setting.GmpaySecret
	setting.GmpayDomain, setting.GmpayPid, setting.GmpaySecret = "", "", ""
	t.Cleanup(func() {
		setting.GmpayDomain, setting.GmpayPid, setting.GmpaySecret = previousDomain, previousPid, previousSecret
	})

	_, err := CreateGmpayTransaction(context.Background(), "invoice-1001", "USD", "12.50", "https://merchant.example/notify")
	assert.ErrorIs(t, err, ErrGmpayNotConfigured)
}
