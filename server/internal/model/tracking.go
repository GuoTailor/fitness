package model

import "time"

// ClassFeedback 教练课后反馈（核销单一对一；学员下次上课可见；uk 支持修订覆盖）
type ClassFeedback struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID        int64     `gorm:"column:owner_id;default:1" json:"-"`
	VerificationID int64     `gorm:"uniqueIndex:uk_cf_verify" json:"verificationId"`
	CoachID        int64     `gorm:"index:idx_cf_coach" json:"coachId"`
	UserID         int64     `gorm:"index:idx_cf_user" json:"userId"`
	Content        string    `gorm:"type:text" json:"content"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (ClassFeedback) TableName() string { return "class_feedback" }

// BodyMetric 体重记录（克整数避免浮点；UNIQUE 同日 upsert）。体重属健康数据，隐私协议覆盖。
type BodyMetric struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     int64     `gorm:"uniqueIndex:uk_bm" json:"userId"`
	RecordDate string    `gorm:"column:record_date;type:date;uniqueIndex:uk_bm" json:"recordDate"`
	WeightG    int       `gorm:"column:weight_g" json:"weightG"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (BodyMetric) TableName() string { return "body_metric" }
