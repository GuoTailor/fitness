package service

// P1 二期 · 批次① 营销收尾单测：限时活动价三方比价 / 赠课券 / 评价奖励发券

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fitness/internal/model"
)

// setPromo 直接落活动价三列（start/end 相对当前时间偏移；走 GORM 保证时间格式与驱动一致）
func setPromo(t *testing.T, db *gorm.DB, skuID, promo int64, startOffset, endOffset time.Duration) {
	t.Helper()
	st, en := time.Now().Add(startOffset), time.Now().Add(endOffset)
	require.NoError(t, db.Model(&model.PackageSKU{}).Where("id = ?", skuID).
		Updates(map[string]interface{}{"promo_price": promo, "promo_start": &st, "promo_end": &en}).Error)
}

// expirePromo 把活动结束时间挪到过去（活动失效）
func expirePromo(t *testing.T, db *gorm.DB, skuID int64) {
	t.Helper()
	past := time.Now().Add(-time.Minute)
	require.NoError(t, db.Model(&model.PackageSKU{}).Where("id = ?", skuID).
		Update("promo_end", &past).Error)
}

// 三方比价：活动价直接生效；窗口未开始不生效
func TestPromoThreeWay(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()

	sku := seedPkg(t, d, 10, 200000)
	setPromo(t, d, sku.ID, 150000, -time.Minute, time.Hour)
	order, err := svc.Create(ctx, 3, sku.ID, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 150000, order.Amount)
	assert.EqualValues(t, 1, order.PromoUsed)
	assert.EqualValues(t, 0, order.NewcomerUsed)

	// 活动未开始（窗口在未来）→ 不生效
	sku2 := seedPkg(t, d, 10, 100000)
	setPromo(t, d, sku2.ID, 60000, time.Hour, 2*time.Hour)
	order2, err := svc.Create(ctx, 3, sku2.ID, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 100000, order2.Amount)
	assert.EqualValues(t, 0, order2.PromoUsed)
}

// 同额时活动价压过新人价（活动不限次，保住新人一次性资格）
func TestPromoTieBeatsNewcomer(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()
	require.NoError(t, d.Model(&model.User{}).Where("id = ?", 2).Update("phone", "13800000002").Error)

	sku := seedPkg(t, d, 10, 200000)
	np := int64(99000)
	require.NoError(t, d.Model(&model.PackageSKU{}).Where("id = ?", sku.ID).Update("newcomer_price", np).Error)
	setPromo(t, d, sku.ID, 99000, -time.Minute, time.Hour) // 与新人价同额 99000

	order, err := svc.Create(ctx, 2, sku.ID, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 99000, order.Amount)
	assert.EqualValues(t, 1, order.PromoUsed)
	assert.EqualValues(t, 0, order.NewcomerUsed, "新人资格未消耗")

	// 活动结束后同一下单人仍可享新人价（资格保留的直接证据）
	expirePromo(t, d, sku.ID)
	order2, err := svc.Create(ctx, 2, sku.ID, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 99000, order2.Amount)
	assert.EqualValues(t, 1, order2.NewcomerUsed)
}

// 赠课券：绑单赠课时、不降价、可与活动价叠加；退款折算剔除赠课
func TestGiftCoupon(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()
	couponSvc := NewCouponService(d)

	tpl := model.CouponTemplate{Name: "赠2课时", Type: model.CouponTypeGift, Amount: 2,
		PerUserLimit: 1, ValidDays: 30, Status: 1}
	require.NoError(t, d.Create(&tpl).Error)
	// 非法参数校验：课时超界 / 带门槛均拒绝
	assert.Error(t, ValidateTemplate(&model.CouponTemplate{Name: "x", Type: model.CouponTypeGift, Amount: 11, ValidDays: 30, PerUserLimit: 1}))
	assert.Error(t, ValidateTemplate(&model.CouponTemplate{Name: "x", Type: model.CouponTypeGift, Amount: 1, Threshold: 100, ValidDays: 30, PerUserLimit: 1}))

	sku := seedPkg(t, d, 10, 200000)
	setPromo(t, d, sku.ID, 150000, -time.Minute, time.Hour)

	uc, err := couponSvc.Claim(ctx, 2, tpl.ID)
	require.NoError(t, err)

	// 可用券列表：赠课券 benefit=0、finalPay=原价、带 giftSessions
	usable, err := couponSvc.UsableForSku(ctx, 2, sku.ID)
	require.NoError(t, err)
	require.Len(t, usable, 1)
	assert.EqualValues(t, 0, usable[0]["benefit"])
	assert.EqualValues(t, 200000, usable[0]["finalPay"])
	assert.EqualValues(t, 2, usable[0]["giftSessions"])

	// 用赠课券下单：活动价照常生效 + 券绑单（叠加，不冲突）
	order, err := svc.Create(ctx, 2, sku.ID, uc.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 150000, order.Amount, "活动价正常叠加")
	assert.EqualValues(t, 0, order.CouponAmount, "赠课券不降价")
	assert.Equal(t, uc.ID, order.CouponID)
	assert.EqualValues(t, 2, order.GiftSessions)

	// mock 支付 → 入账：总课时 = 10 + 2 赠课，独立赠课流水
	_, err = svc.Prepay(ctx, order.ID, 2, "jsapi")
	require.NoError(t, err)
	var mp model.MemberPackage
	require.NoError(t, d.Where("order_id = ?", order.ID).First(&mp).Error)
	assert.Equal(t, 12, mp.TotalSessions)
	assert.Equal(t, 2, mp.GiftSessions)
	var giftTxn model.PackageTransaction
	require.NoError(t, d.Where("idempotency_key = ?", "gift:"+order.OrderNo).First(&giftTxn).Error)
	assert.Equal(t, model.TxnTypeGift, giftTxn.Type)
	assert.Equal(t, 2, giftTxn.SessionsChange)

	// 消耗 3 节后退款折算：按实付金额比例退（可退 7/10，150000×7/10=105000；赠课不参与，P0-2）
	require.NoError(t, d.Model(&model.MemberPackage{}).Where("id = ?", mp.ID).
		Updates(map[string]interface{}{"used_sessions": 3, "status": model.MPStatusActive}).Error)
	refund, err := svc.ApplyRefund(ctx, order.ID, 2, "x")
	require.NoError(t, err)
	approved, err := svc.ApproveRefund(ctx, refund.ID, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 105000, approved.Amount)
	assert.Contains(t, approved.RejectReason, "赠课", "审批备注注明赠课不退")

	// 券返还 usable
	var uc2 model.UserCoupon
	require.NoError(t, d.First(&uc2, uc.ID).Error)
	assert.Equal(t, model.CouponUsable, uc2.Status)
}

// 评价奖励发券：审核通过（课包维度）发放；每人限领由模板控制，教练维度再通过不发第二张
func TestReviewRewardCoupon(t *testing.T) {
	svc, d := newOrderSvc(t)
	_ = svc
	ctx := context.Background()
	couponSvc := NewCouponService(d)
	cfgSvc := NewConfigService(d)
	reviewSvc := NewReviewService(d, cfgSvc).WithRewardCoupons(couponSvc)

	tpl := model.CouponTemplate{Name: "评价奖励", Type: model.CouponTypeReduce, Amount: 1000,
		PerUserLimit: 1, ValidDays: 30, Status: 1}
	require.NoError(t, d.Create(&tpl).Error)
	require.NoError(t, cfgSvc.Set(ctx, model.CfgReviewRewardTemplate, fmt.Sprint(tpl.ID), ""))

	// 人工造一条已核销记录（评价门槛）
	now := time.Now()
	v := model.Verification{BizType: "private", UserID: 2, CoachID: 1,
		Code: "GIFTV1", Status: model.VerifyStatusVerified, VerifiedAt: &now, VerifiedBy: "coach"}
	require.NoError(t, d.Create(&v).Error)

	require.NoError(t, reviewSvc.Submit(ctx, 2, v.ID, 5, 5, "不错", nil))
	var count int64
	d.Model(&model.UserCoupon{}).Where("user_id = ?", 2).Count(&count)
	assert.EqualValues(t, 0, count, "待审核不发")

	// 审核通过课包维度 → 发券；教练维度再通过 → 每人限领 1，静默跳过
	var pkgRow, coachRow model.Review
	require.NoError(t, d.Where("verification_id = ? AND target_type = ?", v.ID, model.ReviewTargetPackage).First(&pkgRow).Error)
	require.NoError(t, d.Where("verification_id = ? AND target_type = ?", v.ID, model.ReviewTargetCoach).First(&coachRow).Error)
	require.NoError(t, reviewSvc.Audit(ctx, pkgRow.ID, "approve"))
	require.NoError(t, reviewSvc.Audit(ctx, coachRow.ID, "approve"))
	d.Model(&model.UserCoupon{}).Where("user_id = ?", 2).Count(&count)
	assert.EqualValues(t, 1, count)
}
