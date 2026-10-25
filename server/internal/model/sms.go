package model

import "time"

// 短信验证码用途
const SmsPurposeLogin = "login"

// SmsVerification 短信验证码（网页端手机号登录）。
// 校验策略：取该手机号最新一条未用记录（新码自动作废旧码）；错误 5 次作废；用后置 used_at 防重放。
type SmsVerification struct {
	ID        int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Phone     string     `gorm:"size:20;index:idx_smsv_phone" json:"phone"`
	Code      string     `gorm:"size:8" json:"-"`
	Purpose   string     `gorm:"size:20;default:login" json:"purpose"`
	Attempts  int        `json:"-"`
	ExpiresAt time.Time  `json:"expiresAt"`
	UsedAt    *time.Time `json:"usedAt"`
	CreatedAt time.Time  `json:"createdAt"`
}

func (SmsVerification) TableName() string { return "sms_verification" }
