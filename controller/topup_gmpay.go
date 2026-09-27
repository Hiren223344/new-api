package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

type GmpayPayRequest struct {
	Amount int64 `json:"amount"`
}

// getGmpayPayMoney converts the user-facing amount to USD for GM Pay payment.
// GM Pay settles in USD-denominated stablecoins, so this must use GmpayUnitPrice
// rather than the Epay Price ratio (local currency per USD) that getPayMoney uses.
func getGmpayPayMoney(amount float64, group string) float64 {
	originalAmount := amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		amount = amount / common.QuotaPerUnit
	}
	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}
	discount := 1.0
	if ds, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(originalAmount)]; ok {
		if ds > 0 {
			discount = ds
		}
	}
	return amount * setting.GmpayUnitPrice * topupGroupRatio * discount
}

// RequestGmpayPay creates a GM Pay crypto top-up order and returns the
// payment_url the frontend redirects the user to.
func RequestGmpayPay(c *gin.Context) {
	if !setting.GmpayEnabled {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "GM Pay 支付未启用"})
		return
	}

	var req GmpayPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	gmpayMinTopup := int64(setting.GmpayMinTopUp)
	if req.Amount < gmpayMinTopup {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", gmpayMinTopup)})
		return
	}
	id := c.GetInt("id")
	if rejectInvalidTopUpQuota(c, id, req.Amount) {
		return
	}

	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	payMoney := getGmpayPayMoney(float64(req.Amount), group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	tradeNo := fmt.Sprintf("GMPAY-%d-%d-%s", id, time.Now().UnixMilli(), common.GetRandomString(6))

	notifyUrl := setting.GmpayNotifyUrl
	if notifyUrl == "" {
		notifyUrl = service.GetCallbackAddress() + "/api/gmpay/webhook"
	}

	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		amount = max(int64(float64(req.Amount)/common.QuotaPerUnit), 1)
	}

	topUp := &model.TopUp{
		UserId:          id,
		Amount:          amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodGmpay,
		PaymentProvider: model.PaymentProviderGmpay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("GM Pay 创建充值订单失败 user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	currency := setting.GmpayCurrency
	if currency == "" {
		currency = "USD"
	}
	order, err := service.CreateGmpayTransaction(c.Request.Context(), tradeNo, currency, strconv.FormatFloat(payMoney, 'f', 2, 64), notifyUrl)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("GM Pay 创建订单失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}
	if order.PaymentUrl == "" {
		logger.LogError(c.Request.Context(), fmt.Sprintf("GM Pay 未返回支付链接 user_id=%d trade_no=%s response=%q", id, tradeNo, common.GetJsonString(order)))
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("GM Pay 充值订单创建成功 user_id=%d trade_no=%s amount=%d money=%.2f payment_url=%q", id, tradeNo, req.Amount, payMoney, order.PaymentUrl))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"payment_url": order.PaymentUrl,
			"order_id":    tradeNo,
		},
	})
}

// GmpayNotify handles the notify_url webhook GM Pay posts on order updates.
func GmpayNotify(c *gin.Context) {
	if !isGmpayWebhookEnabled() {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("GM Pay webhook 被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.Status(http.StatusForbidden)
		return
	}

	var notification map[string]any
	if err := common.DecodeJson(c.Request.Body, &notification); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("GM Pay webhook 解析失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.Status(http.StatusBadRequest)
		return
	}

	params := make(map[string]string, len(notification))
	for k, v := range notification {
		switch value := v.(type) {
		case string:
			params[k] = value
		case float64:
			params[k] = strconv.FormatFloat(value, 'f', -1, 64)
		}
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("GM Pay webhook 收到请求 path=%q client_ip=%s params=%q", c.Request.RequestURI, c.ClientIP(), common.GetJsonString(params)))

	if params["pid"] == "" || params["pid"] != setting.GmpayPid || !service.GmpayVerify(params, setting.GmpaySecret) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("GM Pay webhook 验签失败 path=%q client_ip=%s params=%q", c.Request.RequestURI, c.ClientIP(), common.GetJsonString(params)))
		c.Status(http.StatusBadRequest)
		return
	}

	orderId := params["order_id"]
	status, _ := strconv.Atoi(params["status"])
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("GM Pay webhook 验签成功 order_id=%s status=%d client_ip=%s", orderId, status, c.ClientIP()))

	if status != service.GmpayStatusPaid {
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("GM Pay 订单状态非成功，忽略充值 order_id=%s status=%d client_ip=%s", orderId, status, c.ClientIP()))
		c.Status(http.StatusOK)
		return
	}

	LockOrder(orderId)
	defer UnlockOrder(orderId)

	if err := model.RechargeGmpay(orderId, c.ClientIP()); err != nil {
		switch {
		case errors.Is(err, model.ErrTopUpNotFound):
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("GM Pay 回调订单不存在 order_id=%s client_ip=%s", orderId, c.ClientIP()))
		case errors.Is(err, model.ErrPaymentMethodMismatch):
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("GM Pay 订单支付网关不匹配 order_id=%s client_ip=%s", orderId, c.ClientIP()))
		case errors.Is(err, model.ErrTopUpStatusInvalid):
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("GM Pay 订单状态非法 order_id=%s client_ip=%s", orderId, c.ClientIP()))
		default:
			logger.LogError(c.Request.Context(), fmt.Sprintf("GM Pay 充值处理失败 order_id=%s client_ip=%s error=%q", orderId, c.ClientIP(), err.Error()))
		}
		c.Status(http.StatusOK)
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("GM Pay 充值成功 order_id=%s client_ip=%s", orderId, c.ClientIP()))
	c.Status(http.StatusOK)
}
