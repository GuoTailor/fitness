package model

import "time"

// 老带新裂变状态
const (
	ReferralBound   = 1 // 已绑定，待奖励
	ReferralRewared = 2 // 已发奖励
)

// ReferralRecord 邀请关系（invitee 唯一：一人只能被邀请一次）
type ReferralRecord struct {
	ID         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	InviterID  int64      `gorm:"index:idx_ref_inviter" json:"inviterId"`
	InviteeID  int64      `gorm:"uniqueIndex:uk_ref_invitee" json:"inviteeId"`
	Status     int8       `gorm:"default:1" json:"status"`
	CreatedAt  time.Time  `json:"createdAt"`
	RewardedAt *time.Time `json:"rewardedAt"`
}

func (ReferralRecord) TableName() string { return "referral_record" }
