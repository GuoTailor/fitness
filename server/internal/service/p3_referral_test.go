package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fitness/internal/db"
	"fitness/internal/model"
)

func setupReferralDB(t *testing.T) (*gorm.DB, *ReferralService, *ConfigService) {
	t.Helper()
	d, err := db.OpenSQLite()
	require.NoError(t, err)
	require.NoError(t, d.AutoMigrate(
		&model.User{}, &model.ReferralRecord{}, &model.Order{},
		&model.CouponTemplate{}, &model.UserCoupon{}, &model.AppConfig{},
	))
	cfgSvc := NewConfigService(d)
	svc := NewReferralService(d, NewCouponService(d), cfgSvc)
	return d, svc, cfgSvc
}

func seedRefUser(t *testing.T, d *gorm.DB, id int64) {
	t.Helper()
	require.NoError(t, d.Create(&model.User{ID: id, OpenID: "ou-ref-" + string(rune('a'+id%26))}).Error)
}

// 绑定规则矩阵：正常 / 自邀 / 重复 / 邀请人不存在 / 老客（已有实付订单）
func TestReferralBindMatrix(t *testing.T) {
	d, svc, _ := setupReferralDB(t)
	ctx := context.Background()
	seedRefUser(t, d, 1)
	seedRefUser(t, d, 2)
	seedRefUser(t, d, 3)

	require.NoError(t, svc.Bind(ctx, 2, 1)) // 用户2 被用户1 邀请

	// 自邀
	err := svc.Bind(ctx, 3, 3)
	assert.Contains(t, err.Error(), "不能邀请自己")
	// 重复绑定
	err = svc.Bind(ctx, 2, 3)
	assert.Contains(t, err.Error(), "已绑定过邀请人")
	// 邀请人不存在
	err = svc.Bind(ctx, 3, 99)
	assert.Contains(t, err.Error(), "不存在")
	// 老客：用户4 已有实付订单
	seedRefUser(t, d, 4)
	require.NoError(t, d.Create(&model.Order{OrderNo: "R1", UserID: 4, Status: model.OrderStatusPaid, Amount: 100}).Error)
	err = svc.Bind(ctx, 4, 1)
	assert.Contains(t, err.Error(), "不符合邀请条件")
	// 已退款订单同样算老客
	seedRefUser(t, d, 5)
	require.NoError(t, d.Create(&model.Order{OrderNo: "R2", UserID: 5, Status: model.OrderStatusRefunded, Amount: 100}).Error)
	err = svc.Bind(ctx, 5, 1)
	assert.Contains(t, err.Error(), "不符合邀请条件")
	// 仅待支付/关单不算老客
	seedRefUser(t, d, 6)
	require.NoError(t, d.Create(&model.Order{OrderNo: "R3", UserID: 6, Status: model.OrderStatusPending, Amount: 100}).Error)
	require.NoError(t, svc.Bind(ctx, 6, 1))
}

func TestReferralRewardOnce(t *testing.T) {
	d, svc, cfgSvc := setupReferralDB(t)
	ctx := context.Background()
	seedRefUser(t, d, 1) // 邀请人
	seedRefUser(t, d, 2) // 被邀请人
	require.NoError(t, svc.Bind(ctx, 2, 1))

	tpl := model.CouponTemplate{Name: "裂变奖励券", Type: model.CouponTypeReduce, Amount: 500, Status: 1, PerUserLimit: 1, ValidDays: 30}
	require.NoError(t, d.Create(&tpl).Error)
	require.NoError(t, cfgSvc.Set(ctx, model.CfgReferralInviterTemplate, "1", ""))
	require.NoError(t, cfgSvc.Set(ctx, model.CfgReferralInviteeTemplate, "1", ""))

	svc.RewardOnPaidOrder(ctx, 2)
	var rec model.ReferralRecord
	require.NoError(t, d.First(&rec, "invitee_id = ?", 2).Error)
	assert.EqualValues(t, model.ReferralRewared, rec.Status)
	assert.NotNil(t, rec.RewardedAt)

	// 双方各得一张券
	var countA, countB int64
	require.NoError(t, d.Model(&model.UserCoupon{}).Where("user_id = ?", 1).Count(&countA).Error)
	require.NoError(t, d.Model(&model.UserCoupon{}).Where("user_id = ?", 2).Count(&countB).Error)
	assert.Equal(t, int64(1), countA)
	assert.Equal(t, int64(1), countB)

	// 第二笔订单重复触发：不再发
	svc.RewardOnPaidOrder(ctx, 2)
	require.NoError(t, d.Model(&model.UserCoupon{}).Where("user_id = ?", 1).Count(&countA).Error)
	assert.Equal(t, int64(1), countA)
}

// 绑定超 30 天才首单：不发奖励，关系保留 bound
func TestReferralRewardWindowExpired(t *testing.T) {
	d, svc, _ := setupReferralDB(t)
	ctx := context.Background()
	seedRefUser(t, d, 1)
	seedRefUser(t, d, 2)
	require.NoError(t, svc.Bind(ctx, 2, 1))
	stale := time.Now().AddDate(0, 0, -31)
	require.NoError(t, d.Model(&model.ReferralRecord{}).Where("invitee_id = ?", 2).
		Update("created_at", stale).Error)

	svc.RewardOnPaidOrder(ctx, 2)
	var rec model.ReferralRecord
	require.NoError(t, d.First(&rec, "invitee_id = ?", 2).Error)
	assert.EqualValues(t, model.ReferralBound, rec.Status, "超窗不应标记已奖励")
	var coupons int64
	require.NoError(t, d.Model(&model.UserCoupon{}).Count(&coupons).Error)
	assert.Equal(t, int64(0), coupons)
}

// 未配置奖励模板：闸门照走（标记已奖励），不发券不报错
func TestReferralRewardNoTemplate(t *testing.T) {
	d, svc, _ := setupReferralDB(t)
	ctx := context.Background()
	seedRefUser(t, d, 1)
	seedRefUser(t, d, 2)
	require.NoError(t, svc.Bind(ctx, 2, 1))

	svc.RewardOnPaidOrder(ctx, 2)
	var rec model.ReferralRecord
	require.NoError(t, d.First(&rec, "invitee_id = ?", 2).Error)
	assert.EqualValues(t, model.ReferralRewared, rec.Status)
	var coupons int64
	require.NoError(t, d.Model(&model.UserCoupon{}).Count(&coupons).Error)
	assert.Equal(t, int64(0), coupons)
}
