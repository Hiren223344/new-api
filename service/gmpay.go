package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
)

// GM Pay ("GmpayStatus" in its API docs) reports order status as an integer:
// 1 = waiting for payment, 2 = paid, 3 = closed, 4 = waiting for
// payment-method selection. Only 2 means the funds have actually arrived.
const (
	GmpayStatusWaitingForPayment      = 1
	GmpayStatusPaid                   = 2
	GmpayStatusClosed                 = 3
	GmpayStatusWaitingForMethodSelect = 4
)

var (
	ErrGmpayNotConfigured = errors.New("GM Pay is not configured. Please contact your administrator.")
	ErrGmpayRequestFailed = errors.New("GM Pay request failed. Please try again.")
	ErrGmpayBadSignature  = errors.New("GM Pay signature verification failed")
)

var gmpayHTTPClient = &http.Client{Timeout: 20 * time.Second}

// GmpayOrderData mirrors the GmpayCreateData schema returned (wrapped in a
// GatewayEnvelope) by both the create-transaction and query endpoints.
type GmpayOrderData struct {
	TradeId        string `json:"trade_id"`
	OrderId        string `json:"order_id"`
	Amount         string `json:"amount"`
	Currency       string `json:"currency"`
	ActualAmount   string `json:"actual_amount"`
	ReceiveAddress string `json:"receive_address"`
	Token          string `json:"token"`
	Network        string `json:"network"`
	Status         int    `json:"status"`
	StatusDetail   string `json:"status_detail"`
	ExpirationTime int64  `json:"expiration_time"`
	PaymentUrl     string `json:"payment_url"`
}

type gmpayEnvelope struct {
	StatusCode int             `json:"status_code"`
	Message    string          `json:"message"`
	Data       *GmpayOrderData `json:"data"`
	RequestId  string          `json:"request_id"`
}

// gmpaySign implements the signing scheme documented by GM Pay: exclude the
// signature field and empty values, sort the remaining field names in ASCII
// order, join as key=value pairs with "&", then compute a lowercase-hex
// HMAC-SHA256 using the API secret as the HMAC key. The same scheme also
// verifies the notify_url webhook and the query request.
func gmpaySign(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "signature" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	slices.Sort(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+params[k])
	}
	payload := strings.Join(pairs, "&")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// GmpayVerify recomputes the signature over params (which must include the
// "signature" field received from GM Pay) and reports whether it matches.
func GmpayVerify(params map[string]string, secret string) bool {
	received := params["signature"]
	if received == "" || secret == "" {
		return false
	}
	expected := gmpaySign(params, secret)
	return hmac.Equal([]byte(expected), []byte(received))
}

func gmpayRequest(ctx context.Context, method, path string, params map[string]string) (*GmpayOrderData, error) {
	domain := strings.TrimRight(strings.TrimSpace(setting.GmpayDomain), "/")
	pid := strings.TrimSpace(setting.GmpayPid)
	secret := strings.TrimSpace(setting.GmpaySecret)
	if domain == "" || pid == "" || secret == "" {
		return nil, ErrGmpayNotConfigured
	}

	if params == nil {
		params = map[string]string{}
	}
	params["pid"] = pid
	params["signature"] = gmpaySign(params, secret)

	var request *http.Request
	var err error
	if method == http.MethodGet {
		values := url.Values{}
		for k, v := range params {
			values.Set(k, v)
		}
		request, err = http.NewRequestWithContext(ctx, http.MethodGet, domain+path+"?"+values.Encode(), nil)
	} else {
		body, marshalErr := common.Marshal(params)
		if marshalErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrGmpayRequestFailed, marshalErr)
		}
		request, err = http.NewRequestWithContext(ctx, method, domain+path, strings.NewReader(string(body)))
		if err == nil {
			request.Header.Set("Content-Type", "application/json")
		}
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGmpayRequestFailed, err)
	}

	response, err := gmpayHTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGmpayRequestFailed, err)
	}
	defer response.Body.Close()

	var envelope gmpayEnvelope
	if err := common.DecodeJson(io.LimitReader(response.Body, 1<<20), &envelope); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGmpayRequestFailed, err)
	}
	if envelope.Data == nil {
		return nil, fmt.Errorf("%w: %s", ErrGmpayRequestFailed, envelope.Message)
	}
	return envelope.Data, nil
}

// CreateGmpayTransaction creates an order via POST /payments/gmpay/v1/order/create-transaction.
func CreateGmpayTransaction(ctx context.Context, orderId, currency, amount, notifyUrl string) (*GmpayOrderData, error) {
	return gmpayRequest(ctx, http.MethodPost, "/payments/gmpay/v1/order/create-transaction", map[string]string{
		"order_id":   orderId,
		"currency":   currency,
		"amount":     amount,
		"notify_url": notifyUrl,
	})
}

// QueryGmpayTransaction looks up an order via GET /payments/gmpay/v1/order/query.
// GM Pay's published example only shows "pid" and "signature" on this
// endpoint; "order_id" is included here to identify which order to query,
// matching the field name used everywhere else in the API. If GM Pay expects
// a different identifier (e.g. "trade_id"), this is the only place to change.
func QueryGmpayTransaction(ctx context.Context, orderId string) (*GmpayOrderData, error) {
	return gmpayRequest(ctx, http.MethodGet, "/payments/gmpay/v1/order/query", map[string]string{
		"order_id": orderId,
	})
}
