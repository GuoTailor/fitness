package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fitness/internal/middleware"
	"fitness/internal/model"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// CoachApplyHandler 教练入驻
type CoachApplyHandler struct {
	svc   *service.CoachApplicationService
	audit *service.AuditService // 可为 nil
}

func NewCoachApplyHandler(svc *service.CoachApplicationService) *CoachApplyHandler {
	return &CoachApplyHandler{svc: svc}
}

// WithAudit 挂载审计（审核动作属敏感操作）
func (h *CoachApplyHandler) WithAudit(a *service.AuditService) *CoachApplyHandler {
	h.audit = a
	return h
}

// Submit POST /api/v1/coach-applications 提交入驻申请
func (h *CoachApplyHandler) Submit(c *gin.Context) {
	var req model.CoachApplication
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	// certImages 由前端传 JSON 数组字符串
	rec, err := h.svc.Submit(c.Request.Context(), middleware.UserID(c), &req)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, rec)
}

// Mine GET /api/v1/coach-applications/mine 我的申请状态
func (h *CoachApplyHandler) Mine(c *gin.Context) {
	out, err := h.svc.Mine(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, out)
}

// AdminList GET /admin/api/v1/coach-applications?status=-1&page=&pageSize=
func (h *CoachApplyHandler) AdminList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	status, _ := strconv.Atoi(c.DefaultQuery("status", "-1"))
	items, total, err := h.svc.AdminList(c.Request.Context(), status, page, pageSize)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"list": items, "total": total})
}

// Approve POST /admin/api/v1/coach-applications/:id/approve 通过（自动建档绑定账号）
func (h *CoachApplyHandler) Approve(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rec, err := h.svc.Review(c.Request.Context(), middleware.AdminID(c), id, true, "")
	if err != nil {
		resp.Handle(c, err)
		return
	}
	if h.audit != nil {
		h.audit.Log(c, "coachapply.approve", "coach_application", id, "通过教练入驻申请：申请 ID "+strconv.FormatInt(id, 10))
	}
	resp.OK(c, rec)
}

// Reject POST /admin/api/v1/coach-applications/:id/reject {reason} 驳回
func (h *CoachApplyHandler) Reject(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	rec, err := h.svc.Review(c.Request.Context(), middleware.AdminID(c), id, false, req.Reason)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	if h.audit != nil {
		h.audit.Log(c, "coachapply.reject", "coach_application", id, "驳回教练入驻申请：申请 ID "+strconv.FormatInt(id, 10))
	}
	resp.OK(c, rec)
}
