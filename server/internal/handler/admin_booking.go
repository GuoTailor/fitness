package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"fitness/internal/model"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// AdminBookingHandler 后台：全馆课表 / 代客预约 / 代核销
type AdminBookingHandler struct {
	db      *gorm.DB
	booking *service.BookingService
	verify  *service.VerifyService
	audit   *service.AuditService
}

// WithAudit 装配操作审计（代客操作留痕）
func (h *AdminBookingHandler) WithAudit(a *service.AuditService) *AdminBookingHandler {
	h.audit = a
	return h
}

func NewAdminBookingHandler(db *gorm.DB, b *service.BookingService, v *service.VerifyService) *AdminBookingHandler {
	return &AdminBookingHandler{db: db, booking: b, verify: v}
}

// ScheduleView GET /admin/api/v1/schedule?date= 全馆课表（私教+团课）
func (h *AdminBookingHandler) ScheduleView(c *gin.Context) {
	date := c.DefaultQuery("date", nowDate())
	var bookings []model.Booking
	h.db.Where("booking_date = ? AND status IN ?", date,
		[]string{model.BookingStatusPending, model.BookingStatusConfirmed, model.BookingStatusCompleted}).
		Order("coach_id, start_minute").Find(&bookings)
	var classes []model.GroupClass
	h.db.Where("class_date = ? AND status IN ?", date,
		[]string{model.GCStatusEnrolling, model.GCStatusFulfilled, model.GCStatusClosed, model.GCStatusFinished}).
		Order("start_minute").Find(&classes)

	idsOf := func(fn func(m map[int64]bool)) []int64 {
		m := map[int64]bool{}
		fn(m)
		out := make([]int64, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		return out
	}
	uids := idsOf(func(m map[int64]bool) {
		for _, b := range bookings {
			m[b.UserID] = true
		}
	})
	cids := idsOf(func(m map[int64]bool) {
		for _, b := range bookings {
			m[b.CoachID] = true
		}
		for _, g := range classes {
			m[g.CoachID] = true
		}
	})
	users, coaches := map[int64]model.User{}, map[int64]model.Coach{}
	if len(uids) > 0 {
		var us []model.User
		h.db.Where("id IN ?", uids).Find(&us)
		for _, u := range us {
			users[u.ID] = u
		}
	}
	if len(cids) > 0 {
		var cs []model.Coach
		h.db.Where("id IN ?", cids).Find(&cs)
		for _, co := range cs {
			coaches[co.ID] = co
		}
	}
	bRows := make([]map[string]interface{}, 0, len(bookings))
	for _, b := range bookings {
		bRows = append(bRows, map[string]interface{}{
			"booking": b, "coachName": coaches[b.CoachID].Name,
			"userName": users[b.UserID].Nickname, "userPhone": users[b.UserID].PlainPhone(),
			"startLabel": minuteLabel(b.StartMinute),
		})
	}
	gRows := make([]map[string]interface{}, 0, len(classes))
	for _, g := range classes {
		gRows = append(gRows, map[string]interface{}{
			"class": g, "coachName": coaches[g.CoachID].Name, "startLabel": minuteLabel(g.StartMinute),
		})
	}
	resp.OK(c, gin.H{"date": date, "bookings": bRows, "groupClasses": gRows})
}

// CreateForUser POST /admin/api/v1/bookings 代客预约
func (h *AdminBookingHandler) CreateForUser(c *gin.Context) {
	var req struct {
		UserID          int64  `json:"userId" binding:"required"`
		CoachID         int64  `json:"coachId" binding:"required"`
		MemberPackageID int64  `json:"memberPackageId" binding:"required"`
		Date            string `json:"date" binding:"required"`
		StartMinute     int    `json:"startMinute" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	booking, err := h.booking.Create(c.Request.Context(), req.UserID, req.CoachID,
		req.MemberPackageID, req.Date, req.StartMinute)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	h.audit.Log(c, "booking.proxy", "booking", booking.ID,
		fmt.Sprintf("代客预约：学员 %d 教练 %d %s %d", req.UserID, req.CoachID, req.Date, req.StartMinute))
	resp.OK(c, booking)
}

// Cancel POST /admin/api/v1/bookings/:id/cancel
func (h *AdminBookingHandler) Cancel(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.booking.AdminCancel(c.Request.Context(), id); err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"cancelled": true})
}

// Verify POST /admin/api/v1/verify {code} 前台代核销
func (h *AdminBookingHandler) Verify(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	result, err := h.verify.Scan(c.Request.Context(), req.Code, 0, true)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	h.audit.Log(c, "verify.proxy", "verification", 0,
		fmt.Sprintf("前台代核销 %s（%s，重复=%v）", req.Code, result["userName"], result["alreadyVerified"]))
	resp.OK(c, result)
}
