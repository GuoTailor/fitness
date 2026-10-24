package service

import (
	"context"
	"strconv"

	"gorm.io/gorm"

	"fitness/internal/model"
)

// ConfigService 运营配置（app_config KV）。读取无缓存：P0 流量下每请求一次查询可接受。
type ConfigService struct{ db *gorm.DB }

func NewConfigService(db *gorm.DB) *ConfigService { return &ConfigService{db: db} }

func (s *ConfigService) Get(ctx context.Context, key, def string) string {
	var row model.AppConfig
	if err := s.db.WithContext(ctx).Where("cfg_key = ?", key).First(&row).Error; err != nil {
		return def
	}
	return row.Value
}

func (s *ConfigService) GetInt(ctx context.Context, key string, def int) int {
	v := s.Get(ctx, key, "")
	if n, err := strconv.Atoi(v); err == nil && n >= 0 {
		return n
	}
	return def
}

// Set 后台改配置（upsert）
func (s *ConfigService) Set(ctx context.Context, key, value, remark string) error {
	var row model.AppConfig
	err := s.db.WithContext(ctx).Where("cfg_key = ?", key).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return s.db.WithContext(ctx).Create(&model.AppConfig{Key: key, Value: value, Remark: remark}).Error
	}
	if err != nil {
		return err
	}
	row.Value = value
	if remark != "" {
		row.Remark = remark
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

func (s *ConfigService) All(ctx context.Context) ([]model.AppConfig, error) {
	var rows []model.AppConfig
	err := s.db.WithContext(ctx).Order("cfg_key").Find(&rows).Error
	return rows, err
}
