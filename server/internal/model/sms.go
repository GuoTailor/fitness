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

// SmsSendCounter 短信发送频控计数器（P1-11：单行条件自增原子化，
// 并发请求无法穿透「60s 重发 + 单日上限」两道闸门）。
type SmsSendCounter struct {
	Phone      string     `gorm:"primaryKey;size:20" json:"-"`
	SendDate   string     `gorm:"primaryKey;size:10" json:"-"`
	SendCount  int        `gorm:"default:0" json:"-"`
	LastSentAt *time.Time `json:"-"`
}

func (SmsSendCounter) TableName() string { return "sms_send_counter" }
