package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fitness/internal/resp"
	"fitness/internal/service"
)

// AdminAuditHandler 后台操作审计日志（仅老板可查）
type AdminAuditHandler struct {
	audit *service.AuditService
}

func NewAdminAuditHandler(a *service.AuditService) *AdminAuditHandler {
	return &AdminAuditHandler{audit: a}
}

// List GET /admin/api/v1/audit-logs?adminId=&action=&dateFrom=&dateTo=&page=
func (h *AdminAuditHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	adminID, _ := strconv.ParseInt(c.DefaultQuery("adminId", "0"), 10, 64)
	rows, total, err := h.audit.List(c.Request.Context(), adminID,
		c.Query("action"), c.Query("dateFrom"), c.Query("dateTo"), page, pageSize)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"list": rows, "total": total})
}
