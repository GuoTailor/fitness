package service

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fitness/internal/config"
	"fitness/internal/model"
)

// ---- 上传 ----

var pngHead = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

func pngBytes(n int) []byte {
	b := append([]byte{}, pngHead...)
	for len(b) < n {
		b = append(b, 0x00)
	}
	return b
}

func TestUploadImage(t *testing.T) {
	svc, err := NewUploadService(config.UploadConfig{Dir: t.TempDir(), BaseURL: "http://x.test/uploads", MaxMB: 1})
	require.NoError(t, err)

	url, err := svc.SaveImage(bytes.NewReader(pngBytes(128)), "a.png")
	require.NoError(t, err)
	assert.Contains(t, url, "http://x.test/uploads/images/")

	// 扩展名白名单外
	_, err = svc.SaveImage(bytes.NewReader(pngBytes(16)), "a.gif")
	assert.Error(t, err)
	// 魔数不符（png 头配 .jpg 扩展名）
	_, err = svc.SaveImage(bytes.NewReader(pngBytes(16)), "a.jpg")
	assert.Error(t, err)
	// 超大小（>1MB）
	_, err = svc.SaveImage(bytes.NewReader(pngBytes(1<<20+2)), "big.png")
	assert.Error(t, err)
}

// ---- 优惠券 ----

// 券领取：总量 / 每人限领
func TestCouponClaimLimits(t *testing.T) {
	svc, d := newOrderSvc(t)
	_ = svc
	ctx := context.Background()
	couponSvc := NewCouponService(d)

	tpl := model.CouponTemplate{Name: "满500减50", Type: model.CouponTypeReduce,
		Threshold: 50000, Amount: 5000, TotalCount: 2, PerUserLimit: 1, ValidDays: 30, Status: 1}
	require.NoError(t, d.Create(&tpl).Error)

	uc1, err := couponSvc.Claim(ctx, 2, tpl.ID)
	require.NoError(t, err)
	assert.Equal(t, model.CouponUsable, uc1.Status)
	assert.NotNil(t, uc1.ExpireAt)

	// 同人重复领 → 限领 1 张
	_, err = couponSvc.Claim(ctx, 2, tpl.ID)
	assert.Error(t, err)
	// 第二人可领
	_, err = couponSvc.Claim(ctx, 3, tpl.ID)
	require.NoError(t, err)
	// 总量 2 发完 → 第三人不可领
	_, err = couponSvc.Claim(ctx, 4, tpl.ID)
	assert.Error(t, err)

	var got model.CouponTemplate
	d.First(&got, tpl.ID)
	assert.Equal(t, 2, got.IssuedCount)
}

// 优惠计算：满减门槛/封顶、折扣
func TestCouponBenefit(t *testing.T) {
	reduce := model.CouponTemplate{Type: model.CouponTypeReduce, Threshold: 50000, Amount: 5000}
	assert.EqualValues(t, 0, CouponBenefit(&reduce, 49900), "未达门槛")
	assert.EqualValues(t, 5000, CouponBenefit(&reduce, 200000))
	huge := model.CouponTemplate{Type: model.CouponTypeReduce, Amount: 99999999}
	assert.EqualValues(t, 199999, CouponBenefit(&huge, 200000), "优惠封顶 price-1")

	discount := model.CouponTemplate{Type: model.CouponTypeDiscount, Amount: 8800}
	assert.EqualValues(t, 24000, CouponBenefit(&discount, 200000), "88折")
}

// 券后下单 → mock 支付 → 退款 → 券返还
func TestOrderWithCouponAndRefundReturn(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()
	couponSvc := NewCouponService(d)

	tpl := model.CouponTemplate{Name: "满500减50", Type: model.CouponTypeReduce,
		Threshold: 50000, Amount: 5000, PerUserLimit: 1, ValidDays: 30, Status: 1}
	require.NoError(t, d.Create(&tpl).Error)
	sku := seedPkg(t, d, 10, 200000)

	uc, err := couponSvc.Claim(ctx, 2, tpl.ID)
	require.NoError(t, err)
	// 可用券列表：benefit 与折后价正确
	usable, err := couponSvc.UsableForSku(ctx, 2, sku.ID)
	require.NoError(t, err)
	require.Len(t, usable, 1)
	assert.EqualValues(t, 5000, usable[0]["benefit"])
	assert.EqualValues(t, 195000, usable[0]["finalPay"])

	// 别人的券不可用（隔离：下单时校验 user_id）
	_, err = svc.Create(ctx, 3, sku.ID, uc.ID)
	assert.Error(t, err)

	order, err := svc.Create(ctx, 2, sku.ID, uc.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 195000, order.Amount)
	assert.EqualValues(t, 5000, order.CouponAmount)
	assert.Equal(t, uc.ID, order.CouponID)

	var uc2 model.UserCoupon
	require.NoError(t, d.First(&uc2, uc.ID).Error)
	assert.Equal(t, model.CouponUsed, uc2.Status)
	assert.Equal(t, order.ID, uc2.OrderID)

	// 已用券不可重复用
	_, err = svc.Create(ctx, 2, sku.ID, uc.ID)
	assert.Error(t, err)

	// mock 支付 → 申请退款 → 审批通过 → 券返还 usable、退款额=券后实付
	_, err = svc.Prepay(ctx, order.ID, 2)
	require.NoError(t, err)
	refund, err := svc.ApplyRefund(ctx, order.ID, 2, "不想要了")
	require.NoError(t, err)
	approved, err := svc.ApproveRefund(ctx, refund.ID, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 195000, approved.Amount, "退款金额=券后实付")

	var uc3 model.UserCoupon
	require.NoError(t, d.First(&uc3, uc.ID).Error)
	assert.Equal(t, model.CouponUsable, uc3.Status, "退款后有效期内券返还")

	var o model.Order
	require.NoError(t, d.First(&o, order.ID).Error)
	assert.Equal(t, model.OrderStatusRefunded, o.Status)
}

// 退款时券已过期 → 不返还（expired 留痕）
func TestCouponExpiredNoReturn(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()
	couponSvc := NewCouponService(d)

	tpl := model.CouponTemplate{Name: "立减", Type: model.CouponTypeReduce,
		Amount: 2000, PerUserLimit: 1, ValidDays: 30, Status: 1}
	require.NoError(t, d.Create(&tpl).Error)
	sku := seedPkg(t, d, 10, 200000)

	uc, _ := couponSvc.Claim(ctx, 3, tpl.ID)
	order, err := svc.Create(ctx, 3, sku.ID, uc.ID)
	require.NoError(t, err)
	// 人为把券挪到已过期
	past := time.Now().Add(-time.Hour)
	require.NoError(t, d.Model(&model.UserCoupon{}).Where("id = ?", uc.ID).Update("expire_at", past).Error)

	_, _ = svc.Prepay(ctx, order.ID, 3)
	refund, err := svc.ApplyRefund(ctx, order.ID, 3, "x")
	require.NoError(t, err)
	_, err = svc.ApproveRefund(ctx, refund.ID, 1)
	require.NoError(t, err)

	var uc2 model.UserCoupon
	require.NoError(t, d.First(&uc2, uc.ID).Error)
	assert.Equal(t, model.CouponExpired, uc2.Status, "退款时券已过期不返还")
}

// ---- 新人价 ----

func seedNewcomerSku(t *testing.T, d *gorm.DB) *model.PackageSKU {
	t.Helper()
	sku := seedPkg(t, d, 10, 200000)
	np := int64(99000)
	require.NoError(t, d.Model(&model.PackageSKU{}).Where("id = ?", sku.ID).Update("newcomer_price", np).Error)
	sku.NewcomerPrice = &np
	return sku
}

// 新人价：首次（绑手机号）享新人价，同手机号第二单/未绑手机号/换账号同号不享
func TestNewcomerPrice(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()
	sku := seedNewcomerSku(t, d)

	// 未绑手机号 → 原价
	order0, err := svc.Create(ctx, 1, sku.ID, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 200000, order0.Amount)
	assert.EqualValues(t, 0, order0.NewcomerUsed)

	// 绑手机号首次 → 新人价
	require.NoError(t, d.Model(&model.User{}).Where("id = ?", 2).Update("phone", "13800000002").Error)
	order1, err := svc.Create(ctx, 2, sku.ID, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 99000, order1.Amount)
	assert.EqualValues(t, 1, order1.NewcomerUsed)

	// 同人第二单 → 原价
	order2, err := svc.Create(ctx, 2, sku.ID, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 200000, order2.Amount)

	// 同手机号换账号共享新人价：phone 唯一索引下一号只能绑一个账号，结构性不可能；
	// 换一个新人（不同手机号）仍可享
	require.NoError(t, d.Model(&model.User{}).Where("id = ?", 4).Update("phone", "13800000004").Error)
	order3, err := svc.Create(ctx, 4, sku.ID, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 99000, order3.Amount)
	assert.EqualValues(t, 1, order3.NewcomerUsed)
}

// 新人价与券不叠加取低价：只应用优惠更大者，券保持可用/新人资格保留
func TestNewcomerVsCoupon(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()
	couponSvc := NewCouponService(d)
	sku := seedNewcomerSku(t, d) // 新人省 101000
	require.NoError(t, d.Model(&model.User{}).Where("id = ?", 2).Update("phone", "13800000002").Error)

	// 小额券（减 5000）< 新人优惠 → 用新人价，券保持 usable
	smallTpl := model.CouponTemplate{Name: "小券", Type: model.CouponTypeReduce, Amount: 5000, ValidDays: 30, Status: 1}
	require.NoError(t, d.Create(&smallTpl).Error)
	ucSmall, _ := couponSvc.Claim(ctx, 2, smallTpl.ID)
	order, err := svc.Create(ctx, 2, sku.ID, ucSmall.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 99000, order.Amount)
	assert.EqualValues(t, 1, order.NewcomerUsed)
	assert.EqualValues(t, 0, order.CouponID, "券未核销")
	var uc1 model.UserCoupon
	require.NoError(t, d.First(&uc1, ucSmall.ID).Error)
	assert.Equal(t, model.CouponUsable, uc1.Status)

	// 同手机号已用过新人价，再买只可用券
	bigTpl := model.CouponTemplate{Name: "大券", Type: model.CouponTypeReduce, Amount: 30000, ValidDays: 30, Status: 1}
	require.NoError(t, d.Create(&bigTpl).Error)
	ucBig, _ := couponSvc.Claim(ctx, 2, bigTpl.ID)
	order2, err := svc.Create(ctx, 2, sku.ID, ucBig.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 170000, order2.Amount, "券后价")
	assert.EqualValues(t, 0, order2.NewcomerUsed)
	assert.Equal(t, ucBig.ID, order2.CouponID)
}

// ---- 评价图片 ----

func TestReviewImages(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	seedAllDaySchedule(t, f)
	require.NoError(t, f.db.AutoMigrate(&model.Review{}, &model.NotifyLog{}))
	ctx := context.Background()
	review := NewReviewService(f.db, NewConfigService(f.db))

	b, err := f.booking.Create(ctx, f.userID, f.coachID, f.mpID, futureDate(2), 10*60)
	require.NoError(t, err)
	v, err := f.booking.GenCode(ctx, b.ID, f.userID)
	require.NoError(t, err)
	_, err = f.verify.Scan(ctx, v.Code, f.coachID, false)
	require.NoError(t, err)

	// 外链 / 超数量拒绝
	err = review.Submit(ctx, f.userID, v.ID, 5, 5, "x", []string{"https://evil.example/a.jpg"})
	assert.Error(t, err)
	err = review.Submit(ctx, f.userID, v.ID, 5, 5, "x", []string{
		"/uploads/1.jpg", "/uploads/2.jpg", "/uploads/3.jpg", "/uploads/4.jpg", "/uploads/5.jpg"})
	assert.Error(t, err)

	// 4 张站内图 OK，列表输出解析后的数组
	images := []string{"/uploads/1.jpg", "/uploads/2.jpg", "/uploads/3.jpg", "/uploads/4.jpg"}
	require.NoError(t, review.Submit(ctx, f.userID, v.ID, 4, 5, "带图评价", images))
	var r model.Review
	require.NoError(t, f.db.Where("target_type = ?", model.ReviewTargetCoach).First(&r).Error)
	require.NoError(t, review.Audit(ctx, r.ID, "approve"))
	items, _, err := review.ListApproved(ctx, model.ReviewTargetCoach, f.coachID, 1, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, images, items[0]["images"])
}
