package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"fitness/internal/service"
	"fitness/internal/wechat"
)

// PayHandler 微信支付回调（公共路由，验签即认证——非 JWT）
type PayHandler struct {
	orders *service.OrderService
	pay    wechat.PaymentProvider
}

func NewPayHandler(o *service.OrderService, p wechat.PaymentProvider) *PayHandler {
	return &PayHandler{orders: o, pay: p}
}

// Notify POST /api/v1/pay/notify
// 验签 + 解密 → HandlePaySuccess（条件 UPDATE 幂等，微信重放安全）
func (h *PayHandler) Notify(c *gin.Context) {
	real, ok := h.pay.(*wechat.RealPay)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "mock 支付通道无回调"})
		return
	}
	nh, err := real.NotifyHandler()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "支付证书未就绪"})
		return
	}
	var transaction struct {
		OutTradeNo    string  `json:"out_trade_no"`
		TransactionId string  `json:"transaction_id"`
		TradeState    string  `json:"trade_state"`
	}
	if _, err := nh.ParseNotifyRequest(c.Request.Context(), c.Request, &transaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "验签失败"})
		return
	}
	if transaction.TradeState == "SUCCESS" {
		if err := h.orders.HandlePaySuccess(c.Request.Context(),
			transaction.OutTradeNo, transaction.TransactionId, "wechatpay"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "订单处理失败"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
}

// RefundNotify POST /api/v1/pay/refund/notify —— v3 退款结果通知（P0 退款终态由查询兜底，
// 此端点仅应答保持微信侧重试不堆积；接入退款回调推进后可在此更新 refund 终态）
func (h *PayHandler) RefundNotify(c *gin.Context) {
	if _, ok := h.pay.(*wechat.RealPay); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "mock 支付通道无回调"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
}
