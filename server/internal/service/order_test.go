package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fitness/internal/db"
	"fitness/internal/errs"
	"fitness/internal/model"
	"fitness/internal/wechat"
)

func db0() (*gorm.DB, error) { return db.OpenSQLite() }

func setupOrderDB(t *testing.T) *gorm.DB {
	t.Helper()
	d, err := db0()
	require.NoError(t, err)
	require.NoError(t, d.AutoMigrate(
		&model.User{}, &model.CoursePackage{}, &model.PackageSKU{}, &model.CoachPackage{},
		&model.Order{}, &model.Payment{}, &model.Refund{},
		&model.MemberPackage{}, &model.PackageTransaction{}, &model.AppConfig{},
	))
	// Prepay 服务端取 openid（E2），测试需预置用户
	for i := int64(1); i <= 5; i++ {
		require.NoError(t, d.Create(&model.User{ID: i, OpenID: fmt.Sprintf("ou-%d", i)}).Error)
	}
	require.NoError(t, d.Create(&model.User{ID: 999, OpenID: "ou-999"}).Error)
	return d
}

func newOrderSvc(t *testing.T) (*OrderService, *gorm.DB) {
	d := setupOrderDB(t)
	return NewOrderService(d, NewConfigService(d), wechat.NewMockPay()), d
}

func seedPkg(t *testing.T, d *gorm.DB, sessions int, price int64) *model.PackageSKU {
	t.Helper()
	pkg := model.CoursePackage{OwnerID: 1, Title: "私教10次", Category: model.PkgCategoryPrivate, Status: 1}
	require.NoError(t, d.Create(&pkg).Error)
	sku := model.PackageSKU{PackageID: pkg.ID, Name: "10次卡", Sessions: sessions, ValidDays: 365,
		Price: price, OriginalPrice: price, Status: 1}
	require.NoError(t, d.Create(&sku).Error)
	return &sku
}

// 全链路：下单 → mock 支付 → 课时入账（含重复回调幂等）
func TestOrderMockPayFlow(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()
	sku := seedPkg(t, d, 10, 200000) // 2000 元 / 10 次 = 200 元每次

	order, err := svc.Create(ctx, 1, sku.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, model.OrderStatusPending, order.Status)
	assert.Equal(t, int64(200000), order.Amount)

	// 未支付不能申请退款
	_, err = svc.ApplyRefund(ctx, order.ID, 1, "不想要了")
	assert.Error(t, err)

	// mock 支付：prepay 直接成功
	res, err := svc.Prepay(ctx, order.ID, 1)
	require.NoError(t, err)
	assert.True(t, res.MockPaid)

	// 订单已支付、member_package 入账、流水存在、快照单价正确
	var got model.Order
	require.NoError(t, d.First(&got, order.ID).Error)
	assert.Equal(t, model.OrderStatusPaid, got.Status)
	assert.NotEmpty(t, got.TransactionID)

	var mp model.MemberPackage
	require.NoError(t, d.Where("order_id = ?", order.ID).First(&mp).Error)
	assert.Equal(t, 10, mp.TotalSessions)
	assert.Equal(t, 0, mp.UsedSessions)
	assert.Equal(t, model.MPStatusInactive, mp.Status, "购入后未开卡")
	assert.Equal(t, int64(20000), mp.UnitPrice, "单课时快照价 = 200000/10")
	assert.Equal(t, 365, mp.ValidDays)

	var txns []model.PackageTransaction
	d.Where("member_package_id = ?", mp.ID).Find(&txns)
	require.Len(t, txns, 1)
	assert.Equal(t, model.TxnTypePurchase, txns[0].Type)

	// 支付成功回调重放（直接调 HandlePaySuccess 两次）：状态不变、不重复入账
	require.NoError(t, svc.HandlePaySuccess(ctx, order.OrderNo, "mocktxn-replay", "replay", order.Amount))
	require.NoError(t, svc.HandlePaySuccess(ctx, order.OrderNo, "mocktxn-replay2", "replay2", order.Amount))
	var cnt int64
	d.Model(&model.MemberPackage{}).Where("order_id = ?", order.ID).Count(&cnt)
	assert.Equal(t, int64(1), cnt, "重复回调不得重复建课时账户")
	var txnCnt int64
	d.Model(&model.PackageTransaction{}).Where("member_package_id = ?", mp.ID).Count(&txnCnt)
	assert.Equal(t, int64(1), txnCnt)

	// 已支付订单不能再 prepay
	_, err = svc.Prepay(ctx, order.ID, 1)
	assert.Error(t, err)
}

// 退款：未开卡 7 天内全额退
func TestRefundFullInactive(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()
	sku := seedPkg(t, d, 10, 200000)

	order, _ := svc.Create(ctx, 2, sku.ID, 0)
	_, err := svc.Prepay(ctx, order.ID, 2)
	require.NoError(t, err)

	refund, err := svc.ApplyRefund(ctx, order.ID, 2, "未开卡全额退")
	require.NoError(t, err)
	assert.Equal(t, model.RefundStatusPending, refund.Status)

	// 重复申请：复用 pending 单
	refund2, err := svc.ApplyRefund(ctx, order.ID, 2, "再点一次")
	require.NoError(t, err)
	assert.Equal(t, refund.ID, refund2.ID)

	got, err := svc.ApproveRefund(ctx, refund.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(200000), got.Amount, "未开卡7天内应全额退")

	var o model.Order
	d.First(&o, order.ID)
	assert.Equal(t, model.OrderStatusRefunded, o.Status)
	var mp model.MemberPackage
	d.Where("order_id = ?", order.ID).First(&mp)
	assert.Equal(t, model.MPStatusRefunded, mp.Status)
	// 重复审批被拒
	_, err = svc.ApproveRefund(ctx, refund.ID, 1)
	assert.Error(t, err)
}

// 退款：已消耗 4 课时 → 剩余 6 × 200 元折算
func TestRefundPartialActivated(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()
	sku := seedPkg(t, d, 10, 200000)

	order, _ := svc.Create(ctx, 3, sku.ID, 0)
	_, err := svc.Prepay(ctx, order.ID, 3)
	require.NoError(t, err)

	// 模拟已开卡并消耗 4 课时
	var mp model.MemberPackage
	require.NoError(t, d.Where("order_id = ?", order.ID).First(&mp).Error)
	require.NoError(t, d.Model(&mp).Updates(map[string]interface{}{
		"status": model.MPStatusActive, "used_sessions": 4,
		"activated_at": time.Now(), "expire_date": time.Now().AddDate(0, 0, 300),
	}).Error)

	refund, err := svc.ApplyRefund(ctx, order.ID, 3, "太忙了没空练")
	require.NoError(t, err)
	got, err := svc.ApproveRefund(ctx, refund.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(6*20000), got.Amount, "剩余6课时 × 200元折算")

	var mp2 model.MemberPackage
	d.First(&mp2, mp.ID)
	assert.Equal(t, 10, mp2.UsedSessions, "退款回收后剩余课时清零")
	assert.Equal(t, model.MPStatusRefunded, mp2.Status)

	// 课时用完再申请退款 → 审批时拒绝
	order2, _ := svc.Create(ctx, 3, sku.ID, 0)
	_, _ = svc.Prepay(ctx, order2.ID, 3)
	var mp3 model.MemberPackage
	d.Where("order_id = ?", order2.ID).First(&mp3)
	d.Model(&mp3).Updates(map[string]interface{}{"status": model.MPStatusActive, "used_sessions": 10,
		"activated_at": time.Now()})
	r2, err := svc.ApplyRefund(ctx, order2.ID, 3, "用完了想退")
	require.NoError(t, err)
	_, err = svc.ApproveRefund(ctx, r2.ID, 1)
	assert.Error(t, err)
	var rr model.Refund
	d.First(&rr, r2.ID)
	assert.Equal(t, model.RefundStatusRejected, rr.Status, "无可退金额应自动转为拒绝")
}

// 退款拒绝：订单回到 paid，可再次正常使用
func TestRefundReject(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()
	sku := seedPkg(t, d, 5, 100000)

	order, _ := svc.Create(ctx, 4, sku.ID, 0)
	_, _ = svc.Prepay(ctx, order.ID, 4)
	refund, err := svc.ApplyRefund(ctx, order.ID, 4, "试试退款")
	require.NoError(t, err)

	require.NoError(t, svc.RejectRefund(ctx, refund.ID, 1, "已开卡多次"))
	var o model.Order
	d.First(&o, order.ID)
	assert.Equal(t, model.OrderStatusPaid, o.Status)
	var r model.Refund
	d.First(&r, refund.ID)
	assert.Equal(t, model.RefundStatusRejected, r.Status)

	// 拒绝后可再次申请退款
	_, err = svc.ApplyRefund(ctx, order.ID, 4, "还是想退")
	require.NoError(t, err)
}

// 数据隔离：不能操作别人的订单
func TestOrderIsolation(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()
	sku := seedPkg(t, d, 10, 200000)

	order, _ := svc.Create(ctx, 1, sku.ID, 0)
	_, err := svc.Prepay(ctx, order.ID, 999) // 他人订单
	assert.Error(t, err)
	var ae *errs.AppError
	assert.ErrorAs(t, err, &ae)
	assert.Equal(t, errs.CodeNotFound, ae.Code, "他人订单应报不存在而非无权限，避免越权探测")

	_, err = svc.ApplyRefund(ctx, order.ID, 999, "x")
	assert.Error(t, err)
}

// 后台手动调整：赠课 +2 / 扣课 -1
func TestManualAdjust(t *testing.T) {
	svc, d := newOrderSvc(t)
	_ = svc
	ctx := context.Background()
	sku := seedPkg(t, d, 10, 200000)

	orderSvc := svc
	order, _ := orderSvc.Create(ctx, 5, sku.ID, 0)
	_, _ = orderSvc.Prepay(ctx, order.ID, 5)

	memberSvc := NewMemberService(d)
	var mp model.MemberPackage
	d.Where("order_id = ?", order.ID).First(&mp)

	updated, err := memberSvc.AdjustSessions(ctx, 5, mp.ID, 2, "开业赠送", "admin")
	require.NoError(t, err)
	assert.Equal(t, 12, updated.TotalSessions)
	assert.Equal(t, 0, updated.UsedSessions)

	updated, err = memberSvc.AdjustSessions(ctx, 5, mp.ID, -1, "扣课", "admin")
	require.NoError(t, err)
	assert.Equal(t, 12, updated.TotalSessions)
	assert.Equal(t, 1, updated.UsedSessions)

	// 调整数量为 0 拒绝
	_, err = memberSvc.AdjustSessions(ctx, 5, mp.ID, 0, "x", "admin")
	assert.Error(t, err)
	// 他人课包不可调整（隔离）
	_, err = memberSvc.AdjustSessions(ctx, 999, mp.ID, 1, "x", "admin")
	assert.Error(t, err)
}
