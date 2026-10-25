package service

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
)

// CheckinService 打卡挑战：月度上课达标发券促复购。
// 每人每月限领一次（uk 兜底）；同一模板月度重复发放会撞券模板每人限领，
// 所以发券必须走 Grant（绕过限领），月度一次由 checkin_reward 唯一键保证。
type CheckinService struct {
	db      *gorm.DB
	coupons *CouponService
	cfg     *ConfigService
}

func NewCheckinService(db *gorm.DB, coupons *CouponService, cfg *ConfigService) *CheckinService {
	return &CheckinService{db: db, coupons: coupons, cfg: cfg}
}

// Status 挑战状态（本月进度 + 是否可领取）
func (s *CheckinService) Status(ctx context.Context, userID int64) (map[string]interface{}, error) {
	out := map[string]interface{}{"enabled": false}
	target := s.cfg.GetInt(ctx, model.CfgCheckinTarget, 0)
	if target <= 0 {
		return out, nil
	}
	out["enabled"] = true
	out["target"] = target
	count, err := s.monthCount(ctx, userID, time.Now())
	if err != nil {
		return nil, err
	}
	out["count"] = count
	if remaining := int64(target) - count; remaining > 0 {
		out["remaining"] = remaining
	}
	claimed, err := s.claimed(ctx, userID, time.Now().Format("2006-01"))
	if err != nil {
		return nil, err
	}
	out["claimed"] = claimed
	out["claimable"] = !claimed && count >= int64(target)
	return out, nil
}

// Claim 达标领取本月奖励
func (s *CheckinService) Claim(ctx context.Context, userID int64) error {
	target := s.cfg.GetInt(ctx, model.CfgCheckinTarget, 0)
	if target <= 0 {
		return errs.Biz("打卡挑战未开启")
	}
	tplVal := s.cfg.Get(ctx, model.CfgCheckinRewardTemplate, "")
	if tplVal == "" {
		return errs.Biz("奖励暂未配置，请联系门店")
	}
	count, err := s.monthCount(ctx, userID, time.Now())
	if err != nil {
		return err
	}
	if count < int64(target) {
		return errs.Biz("本月还需上课 " + strconv.Itoa(target-int(count)) + " 次")
	}
	rec := model.CheckinReward{UserID: userID, Period: time.Now().Format("2006-01")}
	if err := s.db.WithContext(ctx).Create(&rec).Error; err != nil {
		// uk_ck_user_period 兜底并发重复领取
		if claimed, _ := s.claimed(ctx, userID, rec.Period); claimed {
			return errs.Biz("本月奖励已领取")
		}
		return err
	}
	if s.coupons == nil {
		return nil
	}
	tplID, err := strconv.ParseInt(tplVal, 10, 64)
	if err != nil || tplID <= 0 {
		log.Printf("[checkin] 奖励模板配置非法 %q，跳过发放", tplVal)
		return nil
	}
	// 发券失败（停发/抢完）仅记日志：领取资格已占用，管理员可后台定向补发
	if _, err := s.coupons.Grant(ctx, userID, tplID); err != nil {
		log.Printf("[checkin] 奖励券发放失败 user=%d tpl=%d: %v", userID, tplID, err)
	}
	return nil
}

// monthCount 本月上课次数（training_record 按核销生成；record_date 为 YYYY-MM-DD 字符串，ISO 格式可字典序比较）
func (s *CheckinService) monthCount(ctx context.Context, userID int64, now time.Time) (int64, error) {
	y, m := now.Year(), now.Month()
	nextY, nextM := y, m+1
	if nextM > 12 {
		nextY, nextM = nextY+1, 1
	}
	start := fmt.Sprintf("%04d-%02d-01", y, m)
	next := fmt.Sprintf("%04d-%02d-01", nextY, nextM)
	var count int64
	err := s.db.WithContext(ctx).Model(&model.TrainingRecord{}).
		Where("user_id = ? AND record_date >= ? AND record_date < ?", userID, start, next).
		Count(&count).Error
	return count, err
}

func (s *CheckinService) claimed(ctx context.Context, userID int64, period string) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&model.CheckinReward{}).
		Where("user_id = ? AND period = ?", userID, period).Count(&n).Error
	return n > 0, err
}
