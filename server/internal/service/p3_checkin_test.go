package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fitness/internal/db"
	"fitness/internal/model"
)

func TestCheckinFlow(t *testing.T) {
	d, err := db.OpenSQLite()
	require.NoError(t, err)
	require.NoError(t, d.AutoMigrate(
		&model.User{}, &model.TrainingRecord{}, &model.CheckinReward{},
		&model.CouponTemplate{}, &model.UserCoupon{}, &model.AppConfig{},
	))
	cfgSvc := NewConfigService(d)
	svc := NewCheckinService(d, NewCouponService(d), cfgSvc)
	ctx := context.Background()
	require.NoError(t, d.Create(&model.User{ID: 1, OpenID: "ou-ck"}).Error)

	// 未配置 target：关闭
	st, err := svc.Status(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, false, st["enabled"])
	assert.Contains(t, svc.Claim(ctx, 1).Error(), "未开启")

	// target=3，只上了 2 次（奖励模板提前配好，未达标先于未配置校验计数）
	require.NoError(t, cfgSvc.Set(ctx, model.CfgCheckinTarget, "3", ""))
	require.NoError(t, d.Create(&model.CouponTemplate{ID: 7, Name: "打卡奖励", Type: model.CouponTypeReduce, Amount: 300, Status: 1, ValidDays: 30}).Error)
	require.NoError(t, cfgSvc.Set(ctx, model.CfgCheckinRewardTemplate, "7", ""))
	today := time.Now()
	require.NoError(t, d.Create(&model.TrainingRecord{UserID: 1, RecordDate: today.Format("2006-01-02"), Project: "私教", Source: "private"}).Error)
	require.NoError(t, d.Create(&model.TrainingRecord{UserID: 1, RecordDate: today.AddDate(0, 0, -1).Format("2006-01-02"), Project: "团课", Source: "group"}).Error)
	st, err = svc.Status(ctx, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 3, st["target"])
	assert.EqualValues(t, 2, st["count"])
	assert.EqualValues(t, 1, st["remaining"])
	assert.Equal(t, false, st["claimable"])
	assert.Contains(t, svc.Claim(ctx, 1).Error(), "还需上课 1 次")

	// 达标：可领取 → 领取 → 券到账
	require.NoError(t, d.Create(&model.TrainingRecord{UserID: 1, RecordDate: today.AddDate(0, 0, -2).Format("2006-01-02"), Project: "私教", Source: "private"}).Error)
	st, _ = svc.Status(ctx, 1)
	assert.Equal(t, true, st["claimable"])
	require.NoError(t, svc.Claim(ctx, 1))
	var coupons int64
	require.NoError(t, d.Model(&model.UserCoupon{}).Where("user_id = ?", 1).Count(&coupons).Error)
	assert.Equal(t, int64(1), coupons)

	// 重复领取拒绝
	assert.Contains(t, svc.Claim(ctx, 1).Error(), "已领取")
	st, _ = svc.Status(ctx, 1)
	assert.Equal(t, true, st["claimed"])

	// 上月领过，本月仍可领（period 独立）
	require.NoError(t, d.Where("user_id = ?", 1).Delete(&model.CheckinReward{}).Error)
	require.NoError(t, d.Create(&model.CheckinReward{UserID: 1, Period: time.Now().AddDate(0, -1, 0).Format("2006-01")}).Error)
	st, _ = svc.Status(ctx, 1)
	assert.Equal(t, true, st["claimable"], "上月记录不占本月资格")
	require.NoError(t, svc.Claim(ctx, 1))
}

// 上课次数统计只算本月（上月的记录不计入）
func TestCheckinMonthBoundary(t *testing.T) {
	d, err := db.OpenSQLite()
	require.NoError(t, err)
	require.NoError(t, d.AutoMigrate(&model.User{}, &model.TrainingRecord{}, &model.CheckinReward{}, &model.AppConfig{}))
	cfgSvc := NewConfigService(d)
	svc := NewCheckinService(d, nil, cfgSvc)
	ctx := context.Background()
	require.NoError(t, d.Create(&model.User{ID: 1, OpenID: "ou-ck2"}).Error)
	require.NoError(t, cfgSvc.Set(ctx, model.CfgCheckinTarget, "2", ""))
	now := time.Now()
	require.NoError(t, d.Create(&model.TrainingRecord{UserID: 1, RecordDate: now.Format("2006-01-02"), Project: "x", Source: "private"}).Error)
	require.NoError(t, d.Create(&model.TrainingRecord{UserID: 1, RecordDate: now.AddDate(0, -1, 0).Format("2006-01-02"), Project: "x", Source: "private"}).Error)
	st, err := svc.Status(ctx, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 1, st["count"], "只统计本月记录")
}
