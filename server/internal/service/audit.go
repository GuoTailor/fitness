package service

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"fitness/internal/middleware"
	"fitness/internal/model"
)

// AuditService 后台操作审计：资金/敏感操作留痕。
// 同步插入、失败仅记日志——审计不阻断业务主流程。
type AuditService struct{ db *gorm.DB }

func NewAuditService(db *gorm.DB) *AuditService { return &AuditService{db: db} }

// Log 记录一条审计日志（操作身份取自 JWT 上下文）
func (s *AuditService) Log(c *gin.Context, action, targetType string, targetID int64, detail string) {
	if s == nil || s.db == nil {
		return
	}
	if len(detail) > 1000 {
		detail = detail[:1000]
	}
	entry := model.AdminAuditLog{
		AdminID:    middleware.AdminID(c),
		Username:   middleware.AdminUsername(c),
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detail,
		IP:         c.ClientIP(),
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&entry).Error; err != nil {
		log.Printf("[audit] 写入失败 action=%s target=%s/%d: %v", action, targetType, targetID, err)
	}
}

// List 审计日志查询（可按操作人/动作/时间过滤）
func (s *AuditService) List(ctx context.Context, adminID int64, action, dateFrom, dateTo string, page, pageSize int) ([]model.AdminAuditLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	q := s.db.WithContext(ctx).Model(&model.AdminAuditLog{})
	if adminID > 0 {
		q = q.Where("admin_id = ?", adminID)
	}
	if action != "" {
		q = q.Where("action = ?", action)
	}
	if dateFrom != "" {
		q = q.Where("created_at >= ?", dateFrom)
	}
	if dateTo != "" {
		q = q.Where("created_at <= ?", dateTo+" 23:59:59")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.AdminAuditLog
	if err := q.Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
