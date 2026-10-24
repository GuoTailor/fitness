package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"fitness/internal/middleware"
	"fitness/internal/model"
	"fitness/internal/resp"
	"fitness/internal/service"
)

func nowDate() string { return time.Now().Format("2006-01-02") }

func addDays(date string, days int) string {
	t, _ := time.ParseInLocation("2006-01-02", date, time.Local)
	return t.AddDate(0, 0, days).Format("2006-01-02")
}

func minuteLabel(m int) string {
	return fmt.Sprintf("%02d:%02d", m/60, m%60)
}

// CoachHandler 教练工作台（同小程序双角色）
type CoachHandler struct {
	db      *gorm.DB
	booking *service.BookingService
	group   *service.GroupService
	verify  *service.VerifyService
	owner   int64
}

func NewCoachHandler(db *gorm.DB, b *service.BookingService, g *service.GroupService, v *service.VerifyService, owner int64) *CoachHandler {
	return &CoachHandler{db: db, booking: b, group: g, verify: v, owner: owner}
}

// Schedule GET /coach/schedule
func (h *CoachHandler) Schedule(c *gin.Context) {
	var rows []model.CoachSchedule
	h.db.Where("coach_id = ?", middleware.CoachID(c)).Order("weekday, start_minute").Find(&rows)
	resp.OK(c, rows)
}

// SaveSchedule PUT /coach/schedule 全量替换周模板
func (h *CoachHandler) SaveSchedule(c *gin.Context) {
	var req struct {
		Items []struct {
			Weekday     int8 `json:"weekday" binding:"required"`
			StartMinute int  `json:"startMinute" binding:"required"`
			EndMinute   int  `json:"endMinute" binding:"required"`
		} `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	for _, it := range req.Items {
		if it.Weekday < 0 || it.Weekday > 6 || it.StartMinute%30 != 0 || it.EndMinute <= it.StartMinute || it.EndMinute > 1440 {
			resp.Handle(c, service.BadParamMsg("时段模板不合法"))
			return
		}
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("coach_id = ?", middleware.CoachID(c)).Delete(&model.CoachSchedule{}).Error; err != nil {
			return err
		}
		for _, it := range req.Items {
			if err := tx.Create(&model.CoachSchedule{
				CoachID: middleware.CoachID(c), Weekday: it.Weekday,
				StartMinute: it.StartMinute, EndMinute: it.EndMinute, Status: 1,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"saved": true})
}

// Day GET /coach/bookings?date=
func (h *CoachHandler) Day(c *gin.Context) {
	date := c.DefaultQuery("date", nowDate())
	list, err := h.booking.CoachDay(c.Request.Context(), middleware.CoachID(c), date)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, list)
}

// Confirm POST /coach/bookings/:id/confirm
func (h *CoachHandler) Confirm(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.booking.Confirm(c.Request.Context(), id, middleware.CoachID(c)); err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"confirmed": true})
}

// RejectBooking POST /coach/bookings/:id/reject（拒绝=取消并返还学员课时）
func (h *CoachHandler) RejectBooking(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.booking.CoachCancel(c.Request.Context(), id, middleware.CoachID(c), true); err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"rejected": true})
}

// PublishGroup POST /coach/group-classes
func (h *CoachHandler) PublishGroup(c *gin.Context) {
	var req struct {
		PackageID   int64  `json:"packageId" binding:"required"`
		Title       string `json:"title" binding:"required"`
		Date        string `json:"date" binding:"required"`
		StartMinute int    `json:"startMinute" binding:"required"`
		EndMinute   int    `json:"endMinute" binding:"required"`
		Location    string `json:"location"`
		Capacity    int    `json:"capacity" binding:"required"`
		Remark      string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	gc, err := h.group.Publish(c.Request.Context(), middleware.CoachID(c), req.PackageID, req.Title,
		req.Date, req.StartMinute, req.EndMinute, req.Location, req.Capacity, req.Remark, h.owner)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gc)
}

// MyGroups GET /coach/group-classes?from=&to=
func (h *CoachHandler) MyGroups(c *gin.Context) {
	list, err := h.group.CoachClasses(c.Request.Context(), middleware.CoachID(c), c.Query("from"), c.Query("to"))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, list)
}

// Enrollments GET /coach/group-classes/:id/enrollments
func (h *CoachHandler) Enrollments(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	list, err := h.group.EnrollmentsOf(c.Request.Context(), id)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, list)
}

// CancelGroup POST /coach/group-classes/:id/cancel
func (h *CoachHandler) CancelGroup(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.group.CancelByCoach(c.Request.Context(), id, middleware.CoachID(c), req.Reason); err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"cancelled": true})
}

// Verify POST /coach/verify {code} 扫码核销（私教/团课通用）
func (h *CoachHandler) Verify(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	result, err := h.verify.Scan(c.Request.Context(), req.Code, middleware.CoachID(c), false)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, result)
}

// MyPackages 教练在售课包（发布团课/核对用）
func (h *CoachHandler) MyPackages(c *gin.Context) {
	var pkgs []model.CoursePackage
	h.db.Joins("JOIN coach_package cp ON cp.package_id = course_package.id").
		Where("cp.coach_id = ? AND course_package.status = 1", middleware.CoachID(c)).
		Find(&pkgs)
	resp.OK(c, pkgs)
}

// Stats GET /coach/stats 本月课时数/学员数/评分
func (h *CoachHandler) Stats(c *gin.Context) {
	coachID := middleware.CoachID(c)
	monthStart := time.Now().Format("2006-01") + "-01"
	var lessons int64
	h.db.Model(&model.TrainingRecord{}).
		Where("coach_id = ? AND record_date >= ?", coachID, monthStart).Count(&lessons)
	var students int64
	h.db.Model(&model.Booking{}).
		Where("coach_id = ? AND status = ? AND booking_date >= ?", coachID, model.BookingStatusCompleted, monthStart).
		Distinct("user_id").Count(&students)
	var coach model.Coach
	h.db.First(&coach, coachID)
	resp.OK(c, gin.H{
		"monthLessons": lessons, "monthStudents": students,
		"ratingAvg": coach.RatingAvg, "ratingCount": coach.RatingCount,
	})
}

// Profile GET/PUT /coach/profile
func (h *CoachHandler) GetProfile(c *gin.Context) {
	var coach model.Coach
	if err := h.db.First(&coach, middleware.CoachID(c)).Error; err != nil {
		resp.Handle(c, service.NotFoundJSON())
		return
	}
	resp.OK(c, coach)
}

func (h *CoachHandler) UpdateProfile(c *gin.Context) {
	var req struct {
		Name         string `json:"name"`
		AvatarURL    string `json:"avatarUrl"`
		Years        int    `json:"years"`
		Specialties  string `json:"specialties"`
		Intro        string `json:"intro"`
		IntroVideo   string `json:"introVideoUrl"`
		CertImages   string `json:"certImages"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	res := h.db.Model(&model.Coach{}).Where("id = ?", middleware.CoachID(c)).Updates(map[string]interface{}{
		"avatar_url": req.AvatarURL, "years": req.Years, "specialties": req.Specialties,
		"intro": req.Intro, "intro_video_url": req.IntroVideo, "cert_images": req.CertImages,
	})
	if res.Error != nil {
		resp.Handle(c, res.Error)
		return
	}
	resp.OK(c, gin.H{"saved": true})
}
