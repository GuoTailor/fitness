package model

import "time"

// 教练入驻申请状态
const (
	CoachApplyPending  = 0
	CoachApplyApproved = 1
	CoachApplyRejected = 2
)

// CoachApplication 教练入驻申请（一人同时最多一个 pending；驳回后可重新提交）
type CoachApplication struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID      int64     `gorm:"column:owner_id;default:1" json:"-"`
	UserID       int64     `gorm:"index:idx_ca_user" json:"userId"`
	Name         string    `gorm:"size:64" json:"name"`
	Gender       int8      `json:"gender"`
	Years        int       `json:"years"`
	Specialties  string    `gorm:"size:255" json:"specialties"`
	CertImages   string    `gorm:"type:text" json:"certImages"` // JSON 数组（站内上传 URL）
	Intro        string    `gorm:"type:text" json:"intro"`
	Phone        string    `gorm:"size:20" json:"phone"`
	Status       int8      `gorm:"default:0;index:idx_ca_status" json:"status"`
	RejectReason string    `gorm:"size:255" json:"rejectReason"`
	AdminID      int64     `gorm:"default:0" json:"adminId"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (CoachApplication) TableName() string { return "coach_application" }
