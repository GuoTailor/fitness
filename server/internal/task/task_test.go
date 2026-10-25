package task

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fitness/internal/config"
	"fitness/internal/db"
	"fitness/internal/model"
	"fitness/internal/wechat"
)

func setupTaskDB(t *testing.T) *gorm.DB {
	t.Helper()
	d, err := db.OpenSQLite()
	require.NoError(t, err)
	require.NoError(t, d.AutoMigrate(
		&model.User{}, &model.Coach{}, &model.CoursePackage{}, &model.PackageSKU{},
		&model.Order{}, &model.Payment{}, &model.Refund{},
		&model.MemberPackage{}, &model.PackageTransaction{}, &model.Booking{},
		&model.GroupClass{}, &model.GroupEnrollment{}, &model.Verification{},
		&model.TrainingRecord{}, &model.Review{}, &model.NotifyLog{}, &model.AppConfig{},
		&model.CouponTemplate{}, &model.UserCoupon{},
	))
	return d
}

// 定时任务：评价邀请（核销满 N 天未评价）+ 课时/有效期预警；重复扫描不重发。
func TestRunnerScans(t *testing.T) {
	d := setupTaskDB(t)
	ctx := context.Background()

	user := model.User{OpenID: "task-user"}
	require.NoError(t, d.Create(&user).Error)
	pkg := model.CoursePackage{OwnerID: 1, Title: "任务课包", Status: 1}
	require.NoError(t, d.Create(&pkg).Error)
	// 剩 1 节（≤阈值 3）且 7 天后到期（≤阈值 14）
	mp := model.MemberPackage{
		OwnerID: 1, UserID: user.ID, PackageID: pkg.ID, PackageTitle: pkg.Title,
		TotalSessions: 5, UsedSessions: 4, Status: model.MPStatusActive,
		ExpireDate: ptrDate(time.Now().AddDate(0, 0, 7)),
	}
	require.NoError(t, d.Create(&mp).Error)
	// 3 天前已核销、未评价 → 评价邀请
	verifiedAt := time.Now().AddDate(0, 0, -3)
	v := model.Verification{
		BizType: "private", UserID: user.ID, MemberPackageID: mp.ID,
		Code: "TASKV1", Status: model.VerifyStatusVerified, VerifiedAt: &verifiedAt,
	}
	require.NoError(t, d.Create(&v).Error)

	r := NewRunner(d, nil, config.NotifyConfig{Mock: true}, wechat.NewMockPay())
	n, err := r.ReviewInvites(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	n, err = r.MemberWarnings(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "课时不足 + 快到期各一条")

	var logs []model.NotifyLog
	d.Order("id").Find(&logs)
	require.Len(t, logs, 3)
	scenes := map[string]bool{}
	for _, l := range logs {
		scenes[l.Scene] = true
	}
	assert.True(t, scenes[model.NotifySceneReviewInvite])
	assert.True(t, scenes[model.NotifySceneSessionsWarning])
	assert.True(t, scenes[model.NotifySceneExpireWarning])

	// 重复扫描：notify_log 唯一键兜底，不再新增
	_, err = r.ReviewInvites(ctx)
	require.NoError(t, err)
	_, err = r.MemberWarnings(ctx)
	require.NoError(t, err)
	var cnt int64
	d.Model(&model.NotifyLog{}).Count(&cnt)
	assert.EqualValues(t, 3, cnt)

	// 已评价后即使没发过邀请也不再推（LEFT JOIN review 过滤）
	// ——本例邀请已发过，构造新核销验证：已评价 → 不推
	v2 := model.Verification{
		BizType: "private", UserID: user.ID, MemberPackageID: mp.ID,
		Code: "TASKV2", Status: model.VerifyStatusVerified, VerifiedAt: &verifiedAt,
	}
	require.NoError(t, d.Create(&v2).Error)
	require.NoError(t, d.Create(&model.Review{
		UserID: user.ID, TargetType: model.ReviewTargetPackage, PackageID: pkg.ID,
		VerificationID: v2.ID, Rating: 5, Content: "x", Status: model.ReviewStatusApproved,
	}).Error)
	n, err = r.ReviewInvites(ctx)
	require.NoError(t, err)
	assert.Zero(t, n, "已评价的核销不再邀请")
}

func ptrDate(t time.Time) *time.Time { return &t }

// 状态推进任务：超时关单 / 课包过期 / 预约过期 / 团课结课 / 退款轮询
func TestRunnerStateAdvances(t *testing.T) {
	d := setupTaskDB(t)
	ctx := context.Background()
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	user := model.User{OpenID: "task-adv-user"}
	require.NoError(t, d.Create(&user).Error)
	pkg := model.CoursePackage{OwnerID: 1, Title: "推进课包", Status: 1}
	require.NoError(t, d.Create(&pkg).Error)

	// 1) 超时挂单（3 小时前创建）
	oldOrder := model.Order{OrderNo: "TSKOLD1", OwnerID: 1, UserID: user.ID, PackageID: pkg.ID,
		Title: "x", Amount: 1000, Status: model.OrderStatusPending, SkuSessions: 1, SkuValidDays: 30, SkuUnitPrice: 1000}
	oldOrder.CreatedAt = time.Now().Add(-3 * time.Hour)
	require.NoError(t, d.Create(&oldOrder).Error)

	// 2) 已过期的生效课包
	expiredMp := model.MemberPackage{OwnerID: 1, UserID: user.ID, PackageID: pkg.ID,
		PackageTitle: pkg.Title, TotalSessions: 5, UsedSessions: 1, Status: model.MPStatusActive,
		ExpireDate: ptrDate(time.Now().AddDate(0, 0, -1))}
	require.NoError(t, d.Create(&expiredMp).Error)

	// 3) 昨天已过课时的预约（未核销）+ 未用核销码
	coach := model.Coach{OwnerID: 1, Name: "推进教练", Status: 1}
	require.NoError(t, d.Create(&coach).Error)
	bk := model.Booking{OwnerID: 1, UserID: user.ID, CoachID: coach.ID, MemberPackageID: expiredMp.ID,
		PackageID: pkg.ID, BookingDate: yesterday, StartMinute: 10 * 60, EndMinute: 11 * 60,
		Status: model.BookingStatusConfirmed, SlotToken: "adv:tok"}
	require.NoError(t, d.Create(&bk).Error)
	code := model.Verification{BizType: "private", BookingID: bk.ID, UserID: user.ID, CoachID: coach.ID,
		MemberPackageID: expiredMp.ID, Code: "ADVEXP1", Status: model.VerifyStatusPending}
	require.NoError(t, d.Create(&code).Error)

	// 4) 昨天已开课未取消的团课
	gc := model.GroupClass{OwnerID: 1, CoachID: coach.ID, PackageID: pkg.ID, Title: "推进团课",
		ClassDate: yesterday, StartMinute: 20 * 60, EndMinute: 21 * 60,
		Capacity: 10, EnrolledCount: 1, Status: model.GCStatusEnrolling}
	require.NoError(t, d.Create(&gc).Error)

	// 5) 处理中的退款单（审批已完成等微信终态）
	refundOrder := model.Order{OrderNo: "TSKRF1", OwnerID: 1, UserID: user.ID, PackageID: pkg.ID,
		Title: "x", Amount: 1000, Status: model.OrderStatusRefund, SkuSessions: 1, SkuValidDays: 30, SkuUnitPrice: 1000}
	require.NoError(t, d.Create(&refundOrder).Error)
	refund := model.Refund{RefundNo: "RFTSK1", OrderID: refundOrder.ID, UserID: user.ID,
		Amount: 900, Reason: "x", Status: model.RefundStatusApproved}
	require.NoError(t, d.Create(&refund).Error)

	r := NewRunner(d, nil, config.NotifyConfig{Mock: true}, wechat.NewMockPay())
	n, err := r.CloseExpiredOrders(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	n, err = r.ExpireMemberPackages(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	n, err = r.ExpireBookings(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	n, err = r.FinishGroupClasses(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	n2, err := r.orders.PollApprovedRefunds(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n2)

	var gotOrder model.Order
	d.First(&gotOrder, oldOrder.ID)
	assert.Equal(t, model.OrderStatusClosed, gotOrder.Status, "超时挂单关单")
	var gotMp model.MemberPackage
	d.First(&gotMp, expiredMp.ID)
	assert.Equal(t, model.MPStatusExpired, gotMp.Status, "课包过期")
	var gotBk model.Booking
	d.First(&gotBk, bk.ID)
	assert.Equal(t, model.BookingStatusExpired, gotBk.Status, "预约过期 no-show")
	assert.Equal(t, model.ReleasedSlotToken(bk.ID), gotBk.SlotToken, "过期释放时段")
	assert.EqualValues(t, 1, gotBk.PenaltySessions, "罚没标记")
	var gotCode model.Verification
	d.First(&gotCode, code.ID)
	assert.Equal(t, model.VerifyStatusExpired, gotCode.Status, "过期作废核销码")
	var gotGc model.GroupClass
	d.First(&gotGc, gc.ID)
	assert.Equal(t, model.GCStatusFinished, gotGc.Status, "团课结课")
	var gotRefund model.Refund
	d.First(&gotRefund, refund.ID)
	assert.Equal(t, model.RefundStatusRefunded, gotRefund.Status, "退款轮询推进终态")
	var rfOrder model.Order
	d.First(&rfOrder, refundOrder.ID)
	assert.Equal(t, model.OrderStatusRefunded, rfOrder.Status)

	// 幂等：重复扫描不重复处理
	n, err = r.CloseExpiredOrders(ctx)
	require.NoError(t, err)
	assert.Zero(t, n)
}

// 续费提醒券：课时不足预警触发自动发券；重复扫描不重发（每人限领由模板控制）
func TestRenewalCoupon(t *testing.T) {
	d := setupTaskDB(t)
	ctx := context.Background()

	user := model.User{OpenID: "renew-user"}
	require.NoError(t, d.Create(&user).Error)
	pkg := model.CoursePackage{OwnerID: 1, Title: "续费包", Status: 1}
	require.NoError(t, d.Create(&pkg).Error)
	mp := model.MemberPackage{
		OwnerID: 1, UserID: user.ID, PackageID: pkg.ID, PackageTitle: pkg.Title,
		TotalSessions: 5, UsedSessions: 4, Status: model.MPStatusActive,
	}
	require.NoError(t, d.Create(&mp).Error)
	tpl := model.CouponTemplate{Name: "续费券", Type: model.CouponTypeReduce, Amount: 2000,
		PerUserLimit: 1, ValidDays: 30, Status: 1}
	require.NoError(t, d.Create(&tpl).Error)

	r := NewRunner(d, nil, config.NotifyConfig{Mock: true}, wechat.NewMockPay())
	require.NoError(t, r.cfg.Set(ctx, model.CfgRenewalReminderTemplate, fmt.Sprint(tpl.ID), ""))

	_, err := r.MemberWarnings(ctx)
	require.NoError(t, err)
	var cnt int64
	d.Model(&model.UserCoupon{}).Where("user_id = ?", user.ID).Count(&cnt)
	assert.EqualValues(t, 1, cnt, "首次预警发放续费券")

	// 再次扫描：预警不重发（notify_log），券也不超发（每人限领）
	_, err = r.MemberWarnings(ctx)
	require.NoError(t, err)
	d.Model(&model.UserCoupon{}).Where("user_id = ?", user.ID).Count(&cnt)
	assert.EqualValues(t, 1, cnt)
}
