package model

import "time"

// AdminAuditLog 后台操作审计日志（资金/敏感操作留痕）
type AdminAuditLog struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	AdminID    int64     `gorm:"index:idx_al_admin" json:"adminId"`
	Username   string    `gorm:"size:50" json:"username"`
	Action     string    `gorm:"size:50;index:idx_al_action" json:"action"`
	TargetType string    `gorm:"size:30" json:"targetType"`
	TargetID   int64     `json:"targetId"`
	Detail     string    `gorm:"size:1000" json:"detail"`
	IP         string    `gorm:"size:45" json:"ip"`
	CreatedAt  time.Time `gorm:"index:idx_al_time" json:"createdAt"`
}

func (AdminAuditLog) TableName() string { return "admin_audit_log" }
