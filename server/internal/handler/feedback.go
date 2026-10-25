package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fitness/internal/middleware"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// FeedbackHandler 教练课后反馈 + 体重记录 + 训练周报
type FeedbackHandler struct {
	feedback *service.FeedbackService
	body     *service.BodyService
	verify   *service.VerifyService
}

func NewFeedbackHandler(f *service.FeedbackService, b *service.BodyService, v *service.VerifyService) *FeedbackHandler {
	return &FeedbackHandler{feedback: f, body: b, verify: v}
}

// Write POST /api/v1/coach/feedback {verificationId, content}
func (h *FeedbackHandler) Write(c *gin.Context) {
	var req struct {
		VerificationID int64  `json:"verificationId" binding:"required"`
		Content        string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	fb, err := h.feedback.Write(c.Request.Context(), middleware.CoachID(c), req.VerificationID, req.Content)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, fb)
}

// CoachList GET /api/v1/coach/feedbacks 教练端近期已核销（附反馈）
func (h *FeedbackHandler) CoachList(c *gin.Context) {
	list, err := h.feedback.CoachVerifications(c.Request.Context(), middleware.CoachID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, list)
}

// Mine GET /api/v1/feedback/mine 学员收到的反馈
func (h *FeedbackHandler) Mine(c *gin.Context) {
	list, err := h.feedback.Mine(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, list)
}

// AddMetric POST /api/v1/body-metrics {date, weightG}
func (h *FeedbackHandler) AddMetric(c *gin.Context) {
	var req struct {
		Date    string `json:"date" binding:"required"`
		WeightG int    `json:"weightG" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	m, err := h.body.AddMetric(c.Request.Context(), middleware.UserID(c), req.Date, req.WeightG)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, m)
}

// ListMetrics GET /api/v1/body-metrics?limit=
func (h *FeedbackHandler) ListMetrics(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "90"))
	rows, err := h.body.ListMetrics(c.Request.Context(), middleware.UserID(c), limit)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, rows)
}

// Report GET /api/v1/training/report?date=
func (h *FeedbackHandler) Report(c *gin.Context) {
	data, err := h.verify.WeeklyReport(c.Request.Context(), middleware.UserID(c), c.Query("date"), h.body)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, data)
}
