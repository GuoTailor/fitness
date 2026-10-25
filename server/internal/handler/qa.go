package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fitness/internal/middleware"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// QaHandler 课程问答区
type QaHandler struct {
	qa *service.QaService
}

func NewQaHandler(q *service.QaService) *QaHandler { return &QaHandler{qa: q} }

// ListOfPackage GET /api/v1/packages/:id/qa?page=（公开）
func (h *QaHandler) ListOfPackage(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	list, total, err := h.qa.ListOfPackage(c.Request.Context(), id, page, pageSize)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"list": list, "total": total})
}

// Ask POST /api/v1/qa/questions {packageId, content, anonymous}
func (h *QaHandler) Ask(c *gin.Context) {
	var req struct {
		PackageID int64  `json:"packageId" binding:"required"`
		Content   string `json:"content" binding:"required"`
		Anonymous bool   `json:"anonymous"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	q, err := h.qa.Ask(c.Request.Context(), middleware.UserID(c), req.PackageID, req.Content, req.Anonymous)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, q)
}

// MyQuestions GET /api/v1/qa/mine
func (h *QaHandler) MyQuestions(c *gin.Context) {
	list, err := h.qa.MyQuestions(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, list)
}

// Answer POST /api/v1/qa/questions/:id/answers {content}（学员端；教练走 /coach/qa/:id/answers 同一服务）
func (h *QaHandler) Answer(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	a, err := h.qa.Answer(c.Request.Context(), middleware.UserID(c), id, req.Content)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, a)
}

// CoachQuestions GET /api/v1/coach/qa
func (h *QaHandler) CoachQuestions(c *gin.Context) {
	list, err := h.qa.CoachQuestions(c.Request.Context(), middleware.CoachID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, list)
}

// CoachAnswer POST /api/v1/coach/qa/:id/answers {content}
func (h *QaHandler) CoachAnswer(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	a, err := h.qa.Answer(c.Request.Context(), middleware.UserID(c), id, req.Content)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, a)
}

// CoachPin POST /api/v1/coach/qa/:id/pin
func (h *QaHandler) CoachPin(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.qa.TogglePinCoach(c.Request.Context(), middleware.CoachID(c), id); err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"toggled": true})
}

// ---- 后台 ----

// AdminList GET /admin/api/v1/qa?status=&keyword=&page=
func (h *QaHandler) AdminList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	list, total, err := h.qa.AdminList(c.Request.Context(), c.Query("status"), c.Query("keyword"), page, pageSize)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"list": list, "total": total})
}

// SetQuestionStatus POST /admin/api/v1/qa/questions/:id/status {status}
func (h *QaHandler) SetQuestionStatus(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	if err := h.qa.SetQuestionStatus(c.Request.Context(), id, req.Status); err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"updated": true})
}

// SetAnswerStatus POST /admin/api/v1/qa/answers/:id/status {status}
func (h *QaHandler) SetAnswerStatus(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	if err := h.qa.SetAnswerStatus(c.Request.Context(), id, req.Status); err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"updated": true})
}

// AdminPin POST /admin/api/v1/qa/questions/:id/pin {pinned}
func (h *QaHandler) AdminPin(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Pinned bool `json:"pinned"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	if err := h.qa.AdminPinQuestion(c.Request.Context(), id, req.Pinned); err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"updated": true})
}
