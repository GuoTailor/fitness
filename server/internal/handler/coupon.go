package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fitness/internal/middleware"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// CouponHandler 学员端优惠券：领券中心 / 领取 / 我的券包 / 可用券
type CouponHandler struct {
	coupons *service.CouponService
}

func NewCouponHandler(c *service.CouponService) *CouponHandler { return &CouponHandler{coupons: c} }

// Center GET /api/v1/coupons/center
func (h *CouponHandler) Center(c *gin.Context) {
	items, err := h.coupons.Center(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"items": items})
}

// Claim POST /api/v1/coupons/:id/claim（id=券模板）
func (h *CouponHandler) Claim(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	uc, err := h.coupons.Claim(c.Request.Context(), middleware.UserID(c), id)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, uc)
}

// Mine GET /api/v1/coupons/mine?status=usable|used|expired
func (h *CouponHandler) Mine(c *gin.Context) {
	items, err := h.coupons.Mine(c.Request.Context(), middleware.UserID(c), c.Query("status"))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"items": items})
}

// Usable GET /api/v1/coupons/usable?skuId=（下单面板用）
func (h *CouponHandler) Usable(c *gin.Context) {
	skuID, err := strconv.ParseInt(c.Query("skuId"), 10, 64)
	if err != nil || skuID <= 0 {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	items, err := h.coupons.UsableForSku(c.Request.Context(), middleware.UserID(c), skuID)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"items": items})
}
