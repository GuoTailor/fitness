package handler

import (
	"github.com/gin-gonic/gin"

	"fitness/internal/errs"
	"fitness/internal/model"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// MiscHandler 协议页等公共内容（后台 app_config 可编辑）
type MiscHandler struct {
	cfg *service.ConfigService
}

func NewMiscHandler(cfg *service.ConfigService) *MiscHandler { return &MiscHandler{cfg: cfg} }

// Agreement GET /api/v1/agreement?type=privacy|user（公开，无登录）
func (h *MiscHandler) Agreement(c *gin.Context) {
	title, key := "隐私政策", model.CfgPrivacyContent
	switch c.DefaultQuery("type", "privacy") {
	case "privacy":
	case "user":
		title, key = "用户协议", model.CfgAgreementContent
	default:
		resp.Handle(c, errs.BadParam("type 仅支持 privacy/user"))
		return
	}
	content := h.cfg.Get(c.Request.Context(), key, "")
	if content == "" {
		content = "正在完善中，如有疑问请联系门店客服。"
	}
	resp.OK(c, gin.H{"title": title, "content": content})
}
