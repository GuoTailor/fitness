package model

import "time"

// CheckinReward 打卡挑战月度奖励记录（user+period 唯一：每人每月一次）
type CheckinReward struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"uniqueIndex:uk_ck_user_period" json:"userId"`
	Period    string    `gorm:"size:7;uniqueIndex:uk_ck_user_period" json:"period"`
	CreatedAt time.Time `json:"createdAt"`
}

func (CheckinReward) TableName() string { return "checkin_reward" }
