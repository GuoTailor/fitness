package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fitness/internal/db"
	"fitness/internal/model"
	"fitness/internal/wechat"
)

// fakePay 可配置退款查询结果的支付通道（测试 ABNORMAL/CLOSED 终态）
type fakePay struct {
	status string
	err    error
}

func (f *fakePay) Prepay(_ context.Context, _, _ string, _ int64, _ string) (*wechat.PrepayResult, error) {
	return &wechat.PrepayResult{MockPaid: true}, nil
}

func (f *fakePay) PrepayNative(_ context.Context, _ string, _ int64, _ string) (*wechat.PrepayResult, error) {
	return &wechat.PrepayResult{MockPaid: true}, nil
}

func (f *fakePay) Refund(_ context.Context, _, _ string, _, _ int64, _ string) error { return nil }

func (f *fakePay) QueryRefund(_ context.Context, _ string) (string, error) {
	return f.status, f.err
}

// TestRefundAbnormalFailsAfterThreshold 连续 3 次 ABNORMAL → failed 转人工、订单回 paid、课时恢复（P1-4）
func TestRefundAbnormalFailsAfterThreshold(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()
	fp := &fakePay{status: "ABNORMAL"}
	svc := NewOrderService(e.db, e.cfg, fp)

	order := model.Order{OrderNo: "ORD-ABN-1", UserID: 1, PackageID: e.pkg.ID, Amount: 20000, Status: model.OrderStatusRefund}
	require.NoError(t, e.db.Create(&order).Error)
	mp := model.MemberPackage{UserID: 1, PackageID: e.pkg.ID, OrderID: order.ID, TotalSessions: 10, UsedSessions: 10, Status: model.MPStatusRefunded}
	require.NoError(t, e.db.Create(&mp).Error)
	r := model.Refund{
		RefundNo: "RF-ABN-1", OrderID: order.ID, UserID: 1, Amount: 20000,
		Status: model.RefundStatusApproved, ReclaimedSessions: 2, PrevMPStatus: model.MPStatusActive,
	}
	require.NoError(t, e.db.Create(&r).Error)

	for i := 0; i < refundAbnormalLimit; i++ {
		_, err := svc.PollApprovedRefunds(ctx)
		require.NoError(t, err)
	}
	var got model.Refund
	require.NoError(t, e.db.First(&got, r.ID).Error)
	assert.Equal(t, model.RefundStatusFailed, got.Status, "连续异常达阈值转人工")
	var o model.Order
	require.NoError(t, e.db.First(&o, order.ID).Error)
	assert.Equal(t, model.OrderStatusPaid, o.Status)
	var m model.MemberPackage
	require.NoError(t, e.db.First(&m, mp.ID).Error)
	assert.Equal(t, 8, m.UsedSessions, "课时恢复")
}

// TestCalcRefundAmountRounding 退款金额按比例 ceil 折算：单价整除为 0 的脏场景不再退 0（P0-2）
func TestCalcRefundAmountRounding(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()

	// 9 分 / 10 节：单价整除后为 0，旧逻辑退 0；新逻辑按比例退 6/10
	order := model.Order{Amount: 9}
	mp := model.MemberPackage{TotalSessions: 10, UsedSessions: 4, Status: model.MPStatusActive}
	calc, err := e.orders.calcRefundAmount(ctx, &order, &mp)
	require.NoError(t, err)
	assert.EqualValues(t, 6, calc.amount, "ceil(9×6/10)=6")
	assert.EqualValues(t, 1, calc.unitPrice, "存证单价 ceil(9/10)=1")

	// 拼团价 66000 / 10 节，用 4 节 → 退 6 节金额 39600
	order2 := model.Order{Amount: 66000}
	calc, err = e.orders.calcRefundAmount(ctx, &order2, &mp)
	require.NoError(t, err)
	assert.EqualValues(t, 39600, calc.amount)

	// 含赠课：总 10 赠 2 用 4 → 可退 4/8 → ceil(9×4/8)=5
	mpGift := model.MemberPackage{TotalSessions: 10, GiftSessions: 2, UsedSessions: 4, Status: model.MPStatusActive}
	calc, err = e.orders.calcRefundAmount(ctx, &order, &mpGift)
	require.NoError(t, err)
	assert.EqualValues(t, 5, calc.amount)

	// 已用完拒绝
	mpUsed := model.MemberPackage{TotalSessions: 10, UsedSessions: 10, Status: model.MPStatusActive}
	_, err = e.orders.calcRefundAmount(ctx, &order2, &mpUsed)
	require.Error(t, err)

	// 全赠课拒绝
	mpAllGift := model.MemberPackage{TotalSessions: 10, GiftSessions: 10, Status: model.MPStatusActive}
	_, err = e.orders.calcRefundAmount(ctx, &order2, &mpAllGift)
	require.Error(t, err)

	// 订单已无余额拒绝
	orderPaid := model.Order{Amount: 66000, RefundAmount: 66000}
	_, err = e.orders.calcRefundAmount(ctx, &orderPaid, &mp)
	require.Error(t, err)
}

// TestListPackagesTotal 列表返回真实总数（X1：total 曾被丢弃成当页条数，前端永远只有 1 页）
func TestListPackagesTotal(t *testing.T) {
	d, err := db.OpenSQLite()
	require.NoError(t, err)
	require.NoError(t, d.AutoMigrate(&model.CoursePackage{}, &model.PackageSKU{}, &model.Review{}))
	shop := NewShopService(d, 1)
	for i := 0; i < 5; i++ {
		pkg := model.CoursePackage{OwnerID: 1, Title: fmt.Sprintf("课包%d", i), Status: 1}
		require.NoError(t, d.Create(&pkg).Error)
		require.NoError(t, d.Create(&model.PackageSKU{
			PackageID: pkg.ID, Name: "10次", Sessions: 10, ValidDays: 90,
			Price: 10000 + int64(i), Status: 1,
		}).Error)
	}
	items, total, err := shop.ListPackages(context.Background(), "", "", "", 1, 2)
	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.EqualValues(t, 5, total, "total 必须是全量总数，不是当页条数")
	assert.NotZero(t, items[0].MinPrice, "批量聚合仍要补齐最低价")
}

// TestApproveRefundGateRollback 审批事务内失败：闸门回滚，退款单保持 pending 可重试（P1-2）
func TestApproveRefundGateRollback(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()
	// 指向不存在订单的退款单：事务内 NotFound 触发整体回滚
	r := model.Refund{RefundNo: "RF-atomic-1", OrderID: 999999, UserID: 1, Amount: 100, Status: model.RefundStatusPending}
	require.NoError(t, e.db.Create(&r).Error)
	_, err := e.orders.ApproveRefund(ctx, r.ID, 1)
	require.Error(t, err)
	var got model.Refund
	require.NoError(t, e.db.First(&got, r.ID).Error)
	assert.Equal(t, model.RefundStatusPending, got.Status, "事务失败必须回滚闸门，不得留下 approved 孤儿")
}

// TestZeroApprovedRefundRolledBack 0 元 approved 单：轮询回退 pending，mock 不得 finalize（P1-2）
func TestZeroApprovedRefundRolledBack(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()
	order := model.Order{OrderNo: "ORD-ZERO-1", UserID: 1, PackageID: e.pkg.ID, Amount: 0, Status: model.OrderStatusRefund}
	require.NoError(t, e.db.Create(&order).Error)
	r := model.Refund{RefundNo: "RF-ZERO-1", OrderID: order.ID, UserID: 1, Amount: 0, Status: model.RefundStatusApproved}
	require.NoError(t, e.db.Create(&r).Error)
	_, err := e.orders.PollApprovedRefunds(ctx)
	require.NoError(t, err)
	var got model.Refund
	require.NoError(t, e.db.First(&got, r.ID).Error)
	assert.Equal(t, model.RefundStatusPending, got.Status, "0 元单不得被 mock 直接 finalize")
	var o model.Order
	require.NoError(t, e.db.First(&o, order.ID).Error)
	assert.Equal(t, model.OrderStatusRefund, o.Status, "订单保持 refunding 待重新审批")
}

// TestFailRefundRestoresSessions 退款持续异常转人工：订单回 paid、课时按快照恢复、退款额回退（P1-4）
func TestFailRefundRestoresSessions(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()
	// 造已审批状态：订单 refunding、课包已回收（used=10/status=refunded）、退款单 approved
	order := model.Order{OrderNo: "ORD-FAIL-1", UserID: 1, PackageID: e.pkg.ID, Amount: 50000, Status: model.OrderStatusRefund, RefundAmount: 30000}
	require.NoError(t, e.db.Create(&order).Error)
	mp := model.MemberPackage{UserID: 1, PackageID: e.pkg.ID, OrderID: order.ID, TotalSessions: 10, UsedSessions: 10, Status: model.MPStatusRefunded}
	require.NoError(t, e.db.Create(&mp).Error)
	r := model.Refund{
		RefundNo: "RF-FAIL-1", OrderID: order.ID, UserID: 1, Amount: 30000,
		Status: model.RefundStatusApproved, ReclaimedSessions: 3, PrevMPStatus: model.MPStatusActive,
	}
	require.NoError(t, e.db.Create(&r).Error)

	require.NoError(t, e.orders.failRefund(ctx, &r, "微信退款持续异常"))

	var gotR model.Refund
	require.NoError(t, e.db.First(&gotR, r.ID).Error)
	assert.Equal(t, model.RefundStatusFailed, gotR.Status)
	var gotO model.Order
	require.NoError(t, e.db.First(&gotO, order.ID).Error)
	assert.Equal(t, model.OrderStatusPaid, gotO.Status)
	assert.EqualValues(t, 0, gotO.RefundAmount, "累计退款额回退")
	var gotM model.MemberPackage
	require.NoError(t, e.db.First(&gotM, mp.ID).Error)
	assert.Equal(t, 7, gotM.UsedSessions, "回收的 3 节课恢复")
	assert.Equal(t, model.MPStatusActive, gotM.Status, "账户状态恢复为审批前")
	var txn model.PackageTransaction
	require.NoError(t, e.db.Where("idempotency_key = ?", "refund_restore:RF-FAIL-1").First(&txn).Error)
	assert.Equal(t, 3, txn.SessionsChange)

	// 幂等：再次失败处理不重复恢复
	require.NoError(t, e.orders.failRefund(ctx, &r, "重复"))
	require.NoError(t, e.db.First(&gotM, mp.ID).Error)
	assert.Equal(t, 7, gotM.UsedSessions)
}

// TestFinalizeRefundIdempotent 并发/重复 finalize 只扣一次销量（P1-4 新发现）
func TestFinalizeRefundIdempotent(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()
	order := model.Order{OrderNo: "ORD-FIN-1", UserID: 1, PackageID: e.pkg.ID, Amount: 10000, Status: model.OrderStatusRefund}
	require.NoError(t, e.db.Create(&order).Error)
	require.NoError(t, e.db.Model(&model.CoursePackage{}).Where("id = ?", e.pkg.ID).
		Update("sold_count", 5).Error)
	r := model.Refund{RefundNo: "RF-FIN-1", OrderID: order.ID, UserID: 1, Amount: 10000, Status: model.RefundStatusApproved}
	require.NoError(t, e.db.Create(&r).Error)

	_, err := e.orders.finalizeRefund(ctx, r.ID, order.ID)
	require.NoError(t, err)
	_, err = e.orders.finalizeRefund(ctx, r.ID, order.ID)
	require.NoError(t, err)

	var pkg model.CoursePackage
	require.NoError(t, e.db.First(&pkg, e.pkg.ID).Error)
	assert.Equal(t, 4, pkg.SoldCount, "重复 finalize 只扣一次销量")
	var o model.Order
	require.NoError(t, e.db.First(&o, order.ID).Error)
	assert.Equal(t, model.OrderStatusRefunded, o.Status)
}

// TestCancelByCoachRetry 已取消团课允许重入补返还，幂等键防重复（P1-16）
func TestCancelByCoachRetry(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	ctx := context.Background()
	gc, err := f.group.Publish(ctx, f.coachID, pkgIDOf(f), "重试取消", futureDate(2), 19*60, 20*60, "大厅", 5, "", 1)
	require.NoError(t, err)
	_, err = f.group.Enroll(ctx, gc.ID, f.userID, f.mpID)
	require.NoError(t, err)

	// 第一次取消成功
	require.NoError(t, f.group.CancelByCoach(ctx, gc.ID, f.coachID, "第一次"))
	var mp model.MemberPackage
	require.NoError(t, f.db.First(&mp, f.mpID).Error)
	assert.Equal(t, 0, mp.UsedSessions, "取消返还课时")

	// 重入：已取消团课允许再次调用，幂等不重复返还（P1-16）
	require.NoError(t, f.group.CancelByCoach(ctx, gc.ID, f.coachID, "重试"))
	require.NoError(t, f.db.First(&mp, f.mpID).Error)
	assert.Equal(t, 0, mp.UsedSessions, "重入不得重复返还")

	var cnt int64
	require.NoError(t, f.db.Model(&model.PackageTransaction{}).
		Where("idempotency_key = ?", fmt.Sprintf("group_cancel:%d:%d", gc.ID, f.userID)).
		Count(&cnt).Error)
	assert.EqualValues(t, 1, cnt, "幂等键保证只返还一次")
}

// TestCoachVerificationsNoFeedback 已核销未写反馈：列表不得 panic，输出空反馈（P0-3）
func TestCoachVerificationsNoFeedback(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	seedAllDaySchedule(t, f)
	ctx := context.Background()
	require.NoError(t, f.db.AutoMigrate(&model.ClassFeedback{}, &model.BodyMetric{}))
	fbSvc := NewFeedbackService(f.db)

	b, err := f.booking.Create(ctx, f.userID, f.coachID, f.mpID, futureDate(2), 10*60)
	require.NoError(t, err)
	v, err := f.booking.GenCode(ctx, b.ID, f.userID)
	require.NoError(t, err)
	_, err = f.verify.Scan(ctx, v.Code, f.coachID, false)
	require.NoError(t, err)

	list, err := fbSvc.CoachVerifications(ctx, f.coachID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	fb := list[0]["feedback"].(map[string]interface{})
	assert.Equal(t, "", fb["content"], "无反馈输出空内容，不得 panic")
	assert.Nil(t, fb["createdAt"])
}

// TestGroupBuySeatAccounting 占席计数：开团占 1、参团 +1、重复参团失败回滚不加
func TestGroupBuySeatAccounting(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()

	o1, err := e.gb.Open(ctx, 1, e.sku.ID)
	require.NoError(t, err)
	var gb model.GroupBuy
	require.NoError(t, e.db.First(&gb, o1.GroupBuyID).Error)
	assert.Equal(t, 1, gb.JoinedCount, "团长开团即占一席")

	_, err = e.gb.Join(ctx, 2, o1.GroupBuyID)
	require.NoError(t, err)
	require.NoError(t, e.db.First(&gb, o1.GroupBuyID).Error)
	assert.Equal(t, 2, gb.JoinedCount)

	_, err = e.gb.Join(ctx, 2, o1.GroupBuyID)
	require.Error(t, err)
	require.NoError(t, e.db.First(&gb, o1.GroupBuyID).Error)
	assert.Equal(t, 2, gb.JoinedCount, "重复参团失败必须整体回滚占席")
}

// TestCreateAutoRefundReuseByStatus 自动退款建单按上一张退款单状态判定：
// 进行中复用、被拒重建、已转人工不再建单（复审：rejected 不得永久堵死自动退款）
func TestCreateAutoRefundReuseByStatus(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()
	order := model.Order{OrderNo: "ORD-REJ-1", UserID: 1, PackageID: e.pkg.ID, Amount: 10000, Status: model.OrderStatusPaid}
	require.NoError(t, e.db.Create(&order).Error)
	old := model.Refund{RefundNo: "RF-REJ-OLD", OrderID: order.ID, UserID: 1, Amount: 10000, Status: model.RefundStatusRejected}
	require.NoError(t, e.db.Create(&old).Error)

	// 上一单被拒 → 新建，不复用
	r, err := e.orders.CreateAutoRefund(ctx, order.ID, 1, "团未成团")
	require.NoError(t, err)
	assert.NotEqual(t, old.ID, r.ID, "被拒退款单不得复用")
	assert.Equal(t, model.RefundStatusPending, r.Status)
	var o model.Order
	require.NoError(t, e.db.First(&o, order.ID).Error)
	assert.Equal(t, model.OrderStatusRefund, o.Status)

	// 进行中 → 复用同一张
	r2, err := e.orders.CreateAutoRefund(ctx, order.ID, 1, "重复调用")
	require.NoError(t, err)
	assert.Equal(t, r.ID, r2.ID, "进行中的退款单复用")

	// 已转人工（failed）→ 不再自动建单
	require.NoError(t, e.db.Model(&model.Refund{}).Where("id = ?", r.ID).
		Update("status", model.RefundStatusFailed).Error)
	require.NoError(t, e.db.Model(&model.Order{}).Where("id = ?", order.ID).
		Update("status", model.OrderStatusPaid).Error)
	_, err = e.orders.CreateAutoRefund(ctx, order.ID, 1, "再次自动退款")
	require.Error(t, err)
	var cnt int64
	require.NoError(t, e.db.Model(&model.Refund{}).Where("order_id = ?", order.ID).Count(&cnt).Error)
	assert.EqualValues(t, 2, cnt, "failed 单不再新建，转人工")
}

// TestGroupBuyLatePayAutoRefund 团已失败后的迟到付款：不入账、自动全额退款（P0-1）
func TestGroupBuyLatePayAutoRefund(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()

	o1, err := e.gb.Open(ctx, 1, e.sku.ID)
	require.NoError(t, err)
	o2, err := e.gb.Join(ctx, 2, o1.GroupBuyID)
	require.NoError(t, err)

	// 团被到期任务置 failed（模拟 CloseGroupBuys 已跑完、迟到单尚未支付）
	require.NoError(t, e.db.Model(&model.GroupBuy{}).Where("id = ?", o1.GroupBuyID).
		Update("status", model.GroupBuyFailed).Error)

	// 迟到付款：不入账，转自动退款
	_, err = e.orders.Prepay(ctx, o2.ID, 2, "jsapi")
	require.NoError(t, err)

	var order model.Order
	require.NoError(t, e.db.First(&order, o2.ID).Error)
	assert.Equal(t, model.OrderStatusRefunded, order.Status)
	assert.Equal(t, int64(66000), order.RefundAmount, "未入账应全额退")

	var mpCount int64
	require.NoError(t, e.db.Model(&model.MemberPackage{}).Where("order_id = ?", o2.ID).Count(&mpCount).Error)
	assert.Zero(t, mpCount, "迟到付款不得入账课时")

	// 无老带新/成团联动副作用：团仍为失败态
	var gb model.GroupBuy
	require.NoError(t, e.db.First(&gb, o1.GroupBuyID).Error)
	assert.EqualValues(t, model.GroupBuyFailed, gb.Status)
}

// TestGroupBuyRefundReleaseSeat 退款终态释放占席，新成员可补位并成团（P0-1 死团修复配套）
func TestGroupBuyRefundReleaseSeat(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()

	o1, err := e.gb.Open(ctx, 1, e.sku.ID) // joined=1
	require.NoError(t, err)
	o2, err := e.gb.Join(ctx, 2, o1.GroupBuyID) // joined=2
	require.NoError(t, err)
	_, err = e.orders.Prepay(ctx, o2.ID, 2, "jsapi")
	require.NoError(t, err)

	// 成员 2 支付后退款 → 释放占席，团仍报名中
	refund, err := e.orders.CreateAutoRefund(ctx, o2.ID, 2, "测试退款")
	require.NoError(t, err)
	_, err = e.orders.ApproveRefund(ctx, refund.ID, 0)
	require.NoError(t, err)

	var gb model.GroupBuy
	require.NoError(t, e.db.First(&gb, o1.GroupBuyID).Error)
	assert.EqualValues(t, model.GroupBuyEnrolling, gb.Status)
	assert.Equal(t, 1, gb.JoinedCount, "退款终态释放占席")

	// 新成员补位并成团
	o3, err := e.gb.Join(ctx, 3, o1.GroupBuyID)
	require.NoError(t, err)
	o4, err := e.gb.Join(ctx, 4, o1.GroupBuyID)
	require.NoError(t, err)
	for _, o := range []*model.Order{o1, o3, o4} {
		_, err := e.orders.Prepay(ctx, o.ID, o.UserID, "jsapi")
		require.NoError(t, err)
	}
	require.NoError(t, e.db.First(&gb, o1.GroupBuyID).Error)
	assert.EqualValues(t, model.GroupBuySuccess, gb.Status, "退款释放的席位可被新成员补上并成团")
}
