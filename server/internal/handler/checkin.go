package handler

import (
	"github.com/gin-gonic/gin"

	"fitness/internal/middleware"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// CheckinHandler 打卡挑战
type CheckinHandler struct {
	svc *service.CheckinService
}

func NewCheckinHandler(svc *service.CheckinService) *CheckinHandler {
	return &CheckinHandler{svc: svc}
}

// Status GET /api/v1/checkin/challenge 挑战状态
func (h *CheckinHandler) Status(c *gin.Context) {
	out, err := h.svc.Status(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, out)
}

// Claim POST /api/v1/checkin/claim 领取本月奖励
func (h *CheckinHandler) Claim(c *gin.Context) {
	if err := h.svc.Claim(c.Request.Context(), middleware.UserID(c)); err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"claimed": true})
}
