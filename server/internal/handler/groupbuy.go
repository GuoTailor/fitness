package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fitness/internal/middleware"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// GroupBuyHandler 拼团
type GroupBuyHandler struct {
	svc *service.GroupBuyService
}

func NewGroupBuyHandler(svc *service.GroupBuyService) *GroupBuyHandler {
	return &GroupBuyHandler{svc: svc}
}

// Open POST /api/v1/group-buys {skuId} 开团，返回待支付订单（走 prepay）
func (h *GroupBuyHandler) Open(c *gin.Context) {
	var req struct {
		SkuID int64 `json:"skuId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	order, err := h.svc.Open(c.Request.Context(), middleware.UserID(c), req.SkuID)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, order)
}

// Join POST /api/v1/group-buys/:id/join 参团，返回待支付订单
func (h *GroupBuyHandler) Join(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	order, err := h.svc.Join(c.Request.Context(), middleware.UserID(c), id)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, order)
}

// ListOpen GET /api/v1/group-buys?skuId= 某 SKU 的进行中团
func (h *GroupBuyHandler) ListOpen(c *gin.Context) {
	skuID, _ := strconv.ParseInt(c.Query("skuId"), 10, 64)
	items, err := h.svc.ListOpen(c.Request.Context(), skuID)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"list": items})
}

// My GET /api/v1/group-buys/mine 我参与的团
func (h *GroupBuyHandler) My(c *gin.Context) {
	items, err := h.svc.MyGroups(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"list": items})
}

// Detail GET /api/v1/group-buys/:id 团详情（分享落地页）
func (h *GroupBuyHandler) Detail(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	out, err := h.svc.Detail(c.Request.Context(), middleware.UserID(c), id)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, out)
}

// AdminList GET /admin/api/v1/group-buys?status=&page=&pageSize= 拼团列表
func (h *GroupBuyHandler) AdminList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	status, _ := strconv.Atoi(c.Query("status"))
	items, total, err := h.svc.AdminList(c.Request.Context(), status, page, pageSize)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"list": items, "total": total})
}

// ActivityInfo GET /api/v1/group-buy-activities/:id 活动落地信息（公开只读，Banner 用）
func (h *GroupBuyHandler) ActivityInfo(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	out, err := h.svc.ActivityPublic(c.Request.Context(), id)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, out)
}
