package service

// 第二轮代码审查修复的回归测试：
// 退款顺序与快照落库（P0-2/3）、回调金额比对与关单应答（M1/M2）、
// SKU 快照入账（M5）、团课退出重报账实一致（P0-4）、核销归属前置（M7）。

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fitness/internal/model"
)

// 退款审批：先抢闸门 → 落库快照 → mock 终态；审批后再点拒绝应被挡
func TestRefundGateOrderAndSnapshot(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()
	sku := seedPkg(t, d, 10, 200000)

	order, _ := svc.Create(ctx, 2, sku.ID, 0)
	_, err := svc.Prepay(ctx, order.ID, 2, "jsapi")
	require.NoError(t, err)
	// 消耗 4 课时 → 剩 6 × 200
	var mp model.MemberPackage
	d.Where("order_id = ?", order.ID).First(&mp)
	d.Model(&mp).Updates(map[string]interface{}{
		"status": model.MPStatusActive, "used_sessions": 4,
		"activated_at": time.Now(), "expire_date": time.Now().AddDate(0, 0, 300),
	})

	refund, err := svc.ApplyRefund(ctx, order.ID, 2, "x")
	require.NoError(t, err)
	got, err := svc.ApproveRefund(ctx, refund.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(6*20000), got.Amount)

	// 审批（已抢闸门）后拒绝 → 冲突；不会出现"微信已退、本地被拒"
	err = svc.RejectRefund(ctx, refund.ID, 1, "晚了")
	assert.Error(t, err)

	// 快照落库：amount 此前从不写库（P0-3）
	var rr model.Refund
	require.NoError(t, d.First(&rr, refund.ID).Error)
	assert.Equal(t, int64(6*20000), rr.Amount, "退款金额必须落库")
	assert.Equal(t, int64(20000), rr.UnitPrice, "折算单价快照")
	assert.Equal(t, 6, rr.ReclaimedSessions, "回收课时快照")
	assert.Equal(t, "remainder", rr.CalcType)
	assert.Equal(t, model.RefundStatusRefunded, rr.Status)
}

// 回调金额不符 → 支付流水置 failed、订单不动；关单后的迟到回调幂等应答
func TestPayAmountMismatchAndClosedAck(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()
	sku := seedPkg(t, d, 10, 200000)

	order, _ := svc.Create(ctx, 1, sku.ID, 0)
	// 金额不符
	err := svc.HandlePaySuccess(ctx, order.OrderNo, "txn-bad", "raw", 1)
	assert.Error(t, err, "金额不符应拒绝")
	var pay model.Payment
	d.Where("order_id = ?", order.ID).First(&pay)
	assert.Equal(t, "failed", pay.Status, "金额不符留痕 failed")
	var o model.Order
	d.First(&o, order.ID)
	assert.Equal(t, model.OrderStatusPending, o.Status, "订单保持待支付")

	// 关单后的迟到回调：幂等应答（nil），不再 500 让微信无限重试
	require.NoError(t, d.Model(&model.Order{}).Where("id = ?", order.ID).
		Update("status", model.OrderStatusClosed).Error)
	assert.NoError(t, svc.HandlePaySuccess(ctx, order.OrderNo, "txn-late", "late", order.Amount))
	d.First(&o, order.ID)
	assert.Equal(t, model.OrderStatusClosed, o.Status, "关单不被迟到回调改写")
}

// SKU 下单后被删：入账按订单快照（M5），不再建 0 课时账户
func TestOrderSkuSnapshotCredit(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()
	sku := seedPkg(t, d, 10, 200000)

	order, err := svc.Create(ctx, 1, sku.ID, 0)
	require.NoError(t, err)
	require.Equal(t, 10, order.SkuSessions, "下单快照课时数")
	// 后台"软删"规格（status=0），随后订单走直连入账路径（模拟极端：物理删除）
	require.NoError(t, d.Unscoped().Delete(&model.PackageSKU{}, sku.ID).Error)

	require.NoError(t, svc.HandlePaySuccess(ctx, order.OrderNo, "txn-snap", "raw", order.Amount))
	var mp model.MemberPackage
	require.NoError(t, d.Where("order_id = ?", order.ID).First(&mp).Error)
	assert.Equal(t, 10, mp.TotalSessions, "按下单快照入账，不因 SKU 删除变 0")
	assert.Equal(t, int64(20000), mp.UnitPrice)
	assert.Equal(t, 365, mp.ValidDays)
}

// 团课退出重报（P0-4）：两个报名世代各自有扣次流水；二次退出正常返还
func TestGroupReEnrollLedgerConsistency(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	seedAllDaySchedule(t, f)
	ctx := context.Background()

	var mp0 model.MemberPackage
	require.NoError(t, f.db.First(&mp0, f.mpID).Error)
	gc, err := f.group.Publish(ctx, f.coachID, mp0.PackageID, "重报测试团课", futureDate(3), 10*60, 11*60, "大厅", 10, "", 1)
	require.NoError(t, err)

	usedOf := func() int {
		var mp model.MemberPackage
		f.db.First(&mp, f.mpID)
		return mp.UsedSessions
	}
	ledgerOf := func() []model.PackageTransaction {
		var list []model.PackageTransaction
		f.db.Where("member_package_id = ? AND type = ?", f.mpID, model.TxnTypeDeduct).Find(&list)
		return list
	}

	// 第一次报名
	_, err = f.group.Enroll(ctx, gc.ID, f.userID, f.mpID)
	require.NoError(t, err)
	require.Equal(t, 1, usedOf())
	// 提前退出 → 返还
	require.NoError(t, f.group.Quit(ctx, gc.ID, f.userID))
	require.Equal(t, 0, usedOf())

	// 第二次报名（复用行，enroll_seq=2）
	_, err = f.group.Enroll(ctx, gc.ID, f.userID, f.mpID)
	require.NoError(t, err)
	assert.Equal(t, 1, usedOf(), "重报后净消耗 1")

	// 两个世代都有扣次流水：修复前第二次报名扣余额无流水
	assert.Len(t, ledgerOf(), 2, "两个报名世代各一条扣次流水")

	// 第二次退出 → 返还且流水存在：修复前撞键静默不返还
	require.NoError(t, f.group.Quit(ctx, gc.ID, f.userID))
	assert.Equal(t, 0, usedOf(), "二次退出课时必须返还")
	var refundCnt int64
	f.db.Model(&model.PackageTransaction{}).
		Where("member_package_id = ? AND type = ?", f.mpID, model.TxnTypeBookingRefund).Count(&refundCnt)
	assert.EqualValues(t, 2, refundCnt, "两个世代各一条返还流水")
}

// M7：已核销码的幂等分支也必须先过归属校验（别家教练不得借幂等拿到学员手机号）
func TestVerifyOwnershipBeforeIdempotent(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	seedAllDaySchedule(t, f)
	ctx := context.Background()

	b, err := f.booking.Create(ctx, f.userID, f.coachID, f.mpID, futureDate(2), 10*60)
	require.NoError(t, err)
	v, err := f.booking.GenCode(ctx, b.ID, f.userID)
	require.NoError(t, err)
	_, err = f.verify.Scan(ctx, v.Code, f.coachID, false)
	require.NoError(t, err)

	// 别家教练重复扫同一已核销码 → Forbidden（而非返回学员信息）
	_, err = f.verify.Scan(ctx, v.Code, f.coachID+999, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "只能核销自己课程的学员")
}
