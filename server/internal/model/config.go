package model

import "time"

// 运营配置键（后台可改，值均存字符串）
const (
	CfgBookingCancelPenaltyHours = "booking.cancel_penalty_hours"
	CfgGroupQuitDeadlineHours    = "group_class.quit_deadline_hours"
	CfgRefundFullDays            = "refund.full_days"
	CfgMemberWarnSessions        = "member.warn_sessions"
	CfgMemberWarnExpireDays      = "member.warn_expire_days"
	CfgReviewInviteAfterDays     = "review.invite_after_days"
	CfgReviewAutoApprove         = "review.auto_approve" // 1=评价免审直接通过
)

// AppConfig 运营配置 KV
type AppConfig struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Key       string    `gorm:"column:cfg_key;size:64;uniqueIndex" json:"key"`
	Value     string    `gorm:"column:cfg_value;size:512" json:"value"`
	Remark    string    `gorm:"size:255" json:"remark"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (AppConfig) TableName() string { return "app_config" }

// 订阅消息场景
const (
	NotifySceneBookingConfirmed = "booking_confirmed" // 教练确认 → 通知学员
	NotifySceneBookingNew       = "booking_new"       // 新预约 → 通知教练
	NotifySceneReviewInvite     = "review_invite"     // 核销次日评价邀请
	NotifySceneSessionsWarning  = "sessions_warning"  // 课时不足
	NotifySceneExpireWarning    = "expire_warning"    // 快到期
)

// NotifyLog 订阅消息发送记录（uk 防重发）
type NotifyLog struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"uniqueIndex:uk_notify" json:"userId"`
	Scene     string    `gorm:"size:32;uniqueIndex:uk_notify" json:"scene"`
	BizID     int64     `gorm:"uniqueIndex:uk_notify" json:"bizId"`
	Status    string    `gorm:"size:8;default:sent" json:"status"`
	ErrMsg    string    `gorm:"size:255" json:"errMsg"`
	CreatedAt time.Time `json:"createdAt"`
}

func (NotifyLog) TableName() string { return "notify_log" }
