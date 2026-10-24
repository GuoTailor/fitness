package handler

import (
	"github.com/gin-gonic/gin"

	"fitness/internal/middleware"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// BookingHandler 学员端预约/团课/训练
type BookingHandler struct {
	booking *service.BookingService
	group   *service.GroupService
	verify  *service.VerifyService
}

func NewBookingHandler(b *service.BookingService, g *service.GroupService, v *service.VerifyService) *BookingHandler {
	return &BookingHandler{booking: b, group: g, verify: v}
}

// Slots GET /api/v1/coaches/:id/slots?date=
func (h *BookingHandler) Slots(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	date := c.Query("date")
	if date == "" {
		date = nowDate()
	}
	slots, err := h.booking.SlotsOfDate(c.Request.Context(), id, date)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"date": date, "slots": slots})
}

// Create POST /api/v1/bookings
func (h *BookingHandler) Create(c *gin.Context) {
	var req struct {
		CoachID          int64  `json:"coachId" binding:"required"`
		MemberPackageID  int64  `json:"memberPackageId" binding:"required"`
		Date             string `json:"date" binding:"required"`
		StartMinute      int    `json:"startMinute" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	booking, err := h.booking.Create(c.Request.Context(), middleware.UserID(c),
		req.CoachID, req.MemberPackageID, req.Date, req.StartMinute)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, booking)
}

// My GET /api/v1/bookings?status=
func (h *BookingHandler) My(c *gin.Context) {
	list, err := h.booking.MyBookings(c.Request.Context(), middleware.UserID(c), c.Query("status"))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, list)
}

// Cancel POST /api/v1/bookings/:id/cancel
func (h *BookingHandler) Cancel(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.booking.UserCancel(c.Request.Context(), id, middleware.UserID(c)); err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"cancelled": true})
}

// Code GET /api/v1/bookings/:id/code
func (h *BookingHandler) Code(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	v, err := h.booking.GenCode(c.Request.Context(), id, middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, v)
}

// GroupList GET /api/v1/group-classes?from=&to=
func (h *BookingHandler) GroupList(c *gin.Context) {
	from := c.DefaultQuery("from", nowDate())
	to := c.DefaultQuery("to", addDays(from, 7))
	list, err := h.group.ListBetween(c.Request.Context(), from, to, middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, list)
}

// GroupDetail GET /api/v1/group-classes/:id
func (h *BookingHandler) GroupDetail(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	data, err := h.group.Detail(c.Request.Context(), id, middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, data)
}

// GroupEnroll POST /api/v1/group-classes/:id/enroll
func (h *BookingHandler) GroupEnroll(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		MemberPackageID int64 `json:"memberPackageId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	ge, err := h.group.Enroll(c.Request.Context(), id, middleware.UserID(c), req.MemberPackageID)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, ge)
}

// GroupQuit POST /api/v1/group-classes/:id/quit
func (h *BookingHandler) GroupQuit(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.group.Quit(c.Request.Context(), id, middleware.UserID(c)); err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"quit": true})
}

// GroupCode GET /api/v1/group-classes/:id/code
func (h *BookingHandler) GroupCode(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	v, err := h.group.GenCode(c.Request.Context(), id, middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, v)
}

// Training GET /api/v1/training?from=&to= 统计+记录
func (h *BookingHandler) Training(c *gin.Context) {
	stats, err := h.verify.TrainingStats(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	records, err := h.verify.TrainingRecords(c.Request.Context(), middleware.UserID(c), c.Query("from"), c.Query("to"))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"stats": stats, "records": records})
}
