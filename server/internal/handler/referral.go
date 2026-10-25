package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fitness/internal/middleware"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// ReferralHandler 老带新裂变
type ReferralHandler struct {
	svc *service.ReferralService
}

func NewReferralHandler(svc *service.ReferralService) *ReferralHandler {
	return &ReferralHandler{svc: svc}
}

// Bind POST /api/v1/referral/bind {inviter} 被邀请人绑定邀请关系
func (h *ReferralHandler) Bind(c *gin.Context) {
	var req struct {
		Inviter int64 `json:"inviter" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	if err := h.svc.Bind(c.Request.Context(), middleware.UserID(c), req.Inviter); err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"bound": true})
}

// Mine GET /api/v1/referral/mine 我的邀请状态
func (h *ReferralHandler) Mine(c *gin.Context) {
	out, err := h.svc.Mine(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, out)
}

// AdminList GET /admin/api/v1/referrals?page=&pageSize= 邀请记录
func (h *ReferralHandler) AdminList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	items, total, err := h.svc.AdminList(c.Request.Context(), page, pageSize)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"list": items, "total": total})
}
