package model

import (
	"fmt"
	"time"
)

// CoachSchedule 教练每周可约时段模板（30 分钟粒度，分钟数为当日 0 点起偏移）
type CoachSchedule struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CoachID     int64     `gorm:"uniqueIndex:uk_schedule" json:"coachId"`
	Weekday     int8      `gorm:"uniqueIndex:uk_schedule" json:"weekday"` // 0=周日 … 6=周六
	StartMinute int       `gorm:"uniqueIndex:uk_schedule" json:"startMinute"`
	EndMinute   int       `json:"endMinute"`
	Status      int8      `gorm:"default:1" json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (CoachSchedule) TableName() string { return "coach_schedule" }

// 私教预约状态
const (
	BookingStatusPending   = "pending"   // 待教练确认
	BookingStatusConfirmed = "confirmed" // 已确认
	BookingStatusCompleted = "completed" // 已核销完成
	BookingStatusCancelled = "cancelled"
	BookingStatusExpired   = "expired"
)

// Booking 私教预约。时段占用经 slot_token 表达：
// 占用（pending/confirmed）= "{coach_id}:{date}:{start_minute}"，
// 释放（cancelled/expired/completed）= "done:{id}"；唯一键兜底并发冲突。
// 所有取消/过期路径必须调用 service 层 releaseSlot 置换 token，否则时段永久锁死。
type Booking struct {
	ID              int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID         int64      `gorm:"column:owner_id;default:1" json:"-"`
	UserID          int64      `gorm:"index:idx_booking_user" json:"userId"`
	CoachID         int64      `gorm:"index:idx_booking_slot_day" json:"coachId"`
	PackageID       int64      `json:"packageId"`
	MemberPackageID int64      `json:"memberPackageId"`
	BookingDate     string     `gorm:"column:booking_date;type:date;index:idx_booking_slot_day;index:idx_booking_date" json:"bookingDate"` // YYYY-MM-DD
	StartMinute     int        `gorm:"index:idx_booking_slot_day" json:"startMinute"`
	EndMinute       int        `json:"endMinute"`
	SlotToken       string     `gorm:"column:slot_token;size:64;uniqueIndex" json:"-"`
	Status          string     `gorm:"size:16;default:pending;index:idx_booking_user;index:idx_booking_date;index:idx_booking_slot_day" json:"status"`
	CancelBy        string     `gorm:"column:cancel_by;size:8" json:"cancelBy"`
	CancelTime      *time.Time `json:"cancelTime"`
	PenaltySessions int        `gorm:"default:0" json:"penaltySessions"`
	Remark          string     `gorm:"size:255" json:"remark"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func (Booking) TableName() string { return "booking" }

// SlotTokenFor 计算占用时段的 token
func SlotTokenFor(coachID int64, date string, startMinute int) string {
	return fmt.Sprintf("%d:%s:%d", coachID, date, startMinute)
}

// ReleasedSlotToken 释放后的 token（按预约 id 区分，历史不冲突）
func ReleasedSlotToken(bookingID int64) string {
	return fmt.Sprintf("done:%d", bookingID)
}

// 团课状态
const (
	GCStatusEnrolling = "enrolling"
	GCStatusFulfilled = "fulfilled" // 满员
	GCStatusClosed    = "closed"    // 停止报名
	GCStatusCancelled = "cancelled"
	GCStatusFinished  = "finished"
)

// GroupClass 团课。报名用「UPDATE … WHERE enrolled_count < capacity」乐观锁防超卖。
type GroupClass struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID       int64     `gorm:"column:owner_id;default:1" json:"-"`
	CoachID       int64     `gorm:"index:idx_gc_coach" json:"coachId"`
	PackageID     int64     `json:"packageId"`
	Title         string    `gorm:"size:128" json:"title"`
	ClassDate     string    `gorm:"column:class_date;type:date;index:idx_gc_date" json:"classDate"`
	StartMinute   int       `json:"startMinute"`
	EndMinute     int       `json:"endMinute"`
	Location      string    `gorm:"size:255" json:"location"`
	Capacity      int       `json:"capacity"`
	EnrolledCount int       `gorm:"default:0" json:"enrolledCount"`
	Status        string    `gorm:"size:16;default:enrolling;index:idx_gc_date" json:"status"`
	CancelReason  string    `gorm:"size:255" json:"cancelReason"`
	Remark        string    `gorm:"size:255" json:"remark"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (GroupClass) TableName() string { return "group_class" }

// 团课报名状态
const (
	GEStatusEnrolled = "enrolled"
	GEStatusQuit     = "quit"
	GEStatusVerified = "verified"
)

// GroupEnrollment 团课报名。uk_ge 决定每人每课仅一行：
// 退出置 quit；再次报名时把本行翻回 enrolled（不另插行，避免撞唯一键）。
type GroupEnrollment struct {
	ID              int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupClassID    int64      `gorm:"uniqueIndex:uk_ge;index:idx_ge_user" json:"groupClassId"`
	UserID          int64      `gorm:"uniqueIndex:uk_ge;index:idx_ge_user" json:"userId"`
	MemberPackageID int64      `json:"memberPackageId"`
	Status          string     `gorm:"size:16;default:enrolled;index:idx_ge_user" json:"status"`
	QuitTime        *time.Time `json:"quitTime"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func (GroupEnrollment) TableName() string { return "group_enrollment" }
