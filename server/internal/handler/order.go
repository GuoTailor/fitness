package handler

import (
	"github.com/gin-gonic/gin"

	"fitness/internal/middleware"
	"fitness/internal/resp"
	"fitness/internal/service"
)

type OrderHandler struct {
	orders *service.OrderService
	member *service.MemberService
}

func NewOrderHandler(orders *service.OrderService, member *service.MemberService) *OrderHandler {
	return &OrderHandler{orders: orders, member: member}
}

// Create POST /api/v1/orders {skuId, couponId}
func (h *OrderHandler) Create(c *gin.Context) {
	var req struct {
		SkuID    int64 `json:"skuId" binding:"required"`
		CouponID int64 `json:"couponId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	order, err := h.orders.Create(c.Request.Context(), middleware.UserID(c), req.SkuID, req.CouponID)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, order)
}

// Prepay POST /api/v1/orders/:id/prepay —— mock 通道直接支付成功并入账。
// openid 一律取登录用户的服务端记录（E2），不再接受客户端传入。
// body 可选 {channel:"native"}：网页端扫码支付（返回 codeUrl 渲染二维码）；
// 缺省 jsapi：小程序端拉起支付。
func (h *OrderHandler) Prepay(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Channel string `json:"channel"`
	}
	_ = c.ShouldBindJSON(&req)
	res, err := h.orders.Prepay(c.Request.Context(), id, middleware.UserID(c), req.Channel)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, res)
}

// Get GET /api/v1/orders/:id —— 订单详情（本人校验），网页端扫码支付轮询用
func (h *OrderHandler) Get(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	order, err := h.orders.MyOrder(c.Request.Context(), id, middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, order)
}

// My GET /api/v1/orders?page=&status=
func (h *OrderHandler) My(c *gin.Context) {
	page, size := pageOf(c)
	list, total, err := h.orders.MyOrders(c.Request.Context(), middleware.UserID(c), page, size, c.Query("status"))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"list": list, "total": total, "page": page, "pageSize": size})
}

// ApplyRefund POST /api/v1/orders/:id/refund {reason}
func (h *OrderHandler) ApplyRefund(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	refund, err := h.orders.ApplyRefund(c.Request.Context(), id, middleware.UserID(c), req.Reason)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, refund)
}

// MyPackages GET /api/v1/member/packages
func (h *OrderHandler) MyPackages(c *gin.Context) {
	list, err := h.member.MyPackages(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, list)
}

// MyTransactions GET /api/v1/member/packages/:id/transactions
func (h *OrderHandler) MyTransactions(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	list, err := h.member.MyTransactions(c.Request.Context(), middleware.UserID(c), id)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, list)
}
