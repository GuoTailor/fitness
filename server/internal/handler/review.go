package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fitness/internal/config"
	"fitness/internal/middleware"
	"fitness/internal/model"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// ReviewHandler 评价（学员提交/查看、教练回复、后台审核）+ 训练日历 + 订阅消息模板下发。
type ReviewHandler struct {
	review *service.ReviewService
	verify *service.VerifyService
	notify config.NotifyConfig
}

func NewReviewHandler(r *service.ReviewService, v *service.VerifyService, notify config.NotifyConfig) *ReviewHandler {
	return &ReviewHandler{review: r, verify: v, notify: notify}
}

// Reviewable GET /api/v1/reviews/reviewable 待评价列表
func (h *ReviewHandler) Reviewable(c *gin.Context) {
	list, err := h.review.Reviewable(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"items": list})
}

// Submit POST /api/v1/reviews {verificationId, packageRating, coachRating, content, images[]}
func (h *ReviewHandler) Submit(c *gin.Context) {
	var req struct {
		VerificationID int64    `json:"verificationId" binding:"required"`
		PackageRating  int      `json:"packageRating" binding:"required,min=1,max=5"`
		CoachRating    int      `json:"coachRating" binding:"required,min=1,max=5"`
		Content        string   `json:"content"`
		Images         []string `json:"images"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	err := h.review.Submit(c.Request.Context(), middleware.UserID(c),
		req.VerificationID, req.PackageRating, req.CoachRating, req.Content, req.Images)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"submitted": true})
}

// Mine GET /api/v1/reviews/mine 我的评价
func (h *ReviewHandler) Mine(c *gin.Context) {
	list, err := h.review.MyReviews(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"items": list})
}

// PackageReviews GET /api/v1/packages/:id/reviews 公开的课包评价
func (h *ReviewHandler) PackageReviews(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	h.listApproved(c, model.ReviewTargetPackage, id)
}

// CoachReviewsPublic GET /api/v1/coaches/:id/reviews 公开的教练评价
func (h *ReviewHandler) CoachReviewsPublic(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	h.listApproved(c, model.ReviewTargetCoach, id)
}

func (h *ReviewHandler) listApproved(c *gin.Context, targetType string, targetID int64) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	items, total, err := h.review.ListApproved(c.Request.Context(), targetType, targetID, page, pageSize)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

// TrainingCalendar GET /api/v1/training/calendar?month=YYYY-MM 训练日历打点
func (h *ReviewHandler) TrainingCalendar(c *gin.Context) {
	month := c.Query("month")
	if month == "" {
		month = nowDate()[:7]
	}
	days, err := h.verify.TrainingCalendar(c.Request.Context(), middleware.UserID(c), month)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"month": month, "days": days})
}

// NotifyTemplates GET /api/v1/notify/templates 订阅消息模板（小程序拉起授权用）。
// 只下发已配置模板 ID 的场景；mock/未配置时为空对象，客户端跳过授权弹窗。
func (h *ReviewHandler) NotifyTemplates(c *gin.Context) {
	templates := map[string]string{}
	for scene, id := range h.notify.Templates {
		if id != "" {
			templates[scene] = id
		}
	}
	resp.OK(c, gin.H{"mock": h.notify.Mock, "templates": templates})
}

// ---- 教练端 ----

// CoachMyReviews GET /api/v1/coach/reviews 我收到的评价（教练维度）
func (h *ReviewHandler) CoachMyReviews(c *gin.Context) {
	list, err := h.review.CoachReviews(c.Request.Context(), middleware.CoachID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"items": list})
}

// CoachReply POST /api/v1/coach/reviews/:id/reply {content}
func (h *ReviewHandler) CoachReply(c *gin.Context) {
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
	if err := h.review.Reply(c.Request.Context(), middleware.CoachID(c), id, req.Content); err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"replied": true})
}

// ---- 管理后台 ----

// AdminList GET /admin/api/v1/reviews?status=&targetType=&page=
func (h *ReviewHandler) AdminList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	items, total, err := h.review.AdminList(c.Request.Context(),
		c.Query("status"), c.Query("targetType"), page, pageSize)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

// AdminAudit POST /admin/api/v1/reviews/:id/audit {action: approve|hide}
func (h *ReviewHandler) AdminAudit(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Action string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	if err := h.review.Audit(c.Request.Context(), id, req.Action); err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"audited": true})
}

// AdminFeatured POST /admin/api/v1/reviews/:id/featured {featured: bool}
func (h *ReviewHandler) AdminFeatured(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Featured bool `json:"featured"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	if err := h.review.SetFeatured(c.Request.Context(), id, req.Featured); err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"updated": true})
}
