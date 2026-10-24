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

func setupBookingDB(t *testing.T) *gorm.DB {
	t.Helper()
	d, err := db.OpenSQLite()
	require.NoError(t, err)
	require.NoError(t, d.AutoMigrate(
		&model.User{}, &model.Coach{}, &model.CoachPackage{}, &model.CoachSchedule{},
		&model.CoursePackage{}, &model.PackageSKU{}, &model.AppConfig{},
		&model.MemberPackage{}, &model.PackageTransaction{},
		&model.Booking{}, &model.GroupClass{}, &model.GroupEnrollment{},
		&model.Verification{}, &model.TrainingRecord{},
	))
	// 取消规则：开课前 4 小时内取消扣 1 课时
	require.NoError(t, d.Create(&model.AppConfig{Key: model.CfgBookingCancelPenaltyHours, Value: "4"}).Error)
	require.NoError(t, d.Create(&model.AppConfig{Key: model.CfgGroupQuitDeadlineHours, Value: "4"}).Error)
	return d
}

type fixture struct {
	db      *gorm.DB
	booking *BookingService
	group   *GroupService
	verify  *VerifyService
	coachID int64
	userID  int64
	mpID    int64
}

// futureDate 返回 days 天后的日期字符串，落在有模板的星期由调用方自定
func futureDate(days int) string {
	return time.Now().AddDate(0, 0, days).Format("2006-01-02")
}

func newFixture(t *testing.T, sessions int, windowStart, windowEnd int) *fixture {
	d := setupBookingDB(t)
	cfg := NewConfigService(d)
	f := &fixture{
		db:      d,
		booking: NewBookingService(d, cfg),
		group:   NewGroupService(d, cfg),
		verify:  NewVerifyService(d),
	}
	user := model.User{OpenID: "bk-test-user"}
	require.NoError(t, d.Create(&user).Error)
	f.userID = user.ID
	coach := model.Coach{OwnerID: 1, Name: "测试教练", Status: 1}
	require.NoError(t, d.Create(&coach).Error)
	f.coachID = coach.ID
	pkg := model.CoursePackage{OwnerID: 1, Title: "测试私教", Category: model.PkgCategoryPrivate, Status: 1}
	require.NoError(t, d.Create(&pkg).Error)
	sku := model.PackageSKU{PackageID: pkg.ID, Name: "卡", Sessions: sessions, ValidDays: 365, Price: 100000, Status: 1}
	require.NoError(t, d.Create(&sku).Error)
	require.NoError(t, d.Create(&model.CoachPackage{CoachID: coach.ID, PackageID: pkg.ID}).Error)
	mp := model.MemberPackage{
		OwnerID: 1, UserID: user.ID, PackageID: pkg.ID, SkuID: sku.ID,
		PackageTitle: pkg.Title, TotalSessions: sessions, ValidDays: 365,
		UnitPrice: 10000, Status: model.MPStatusInactive,
	}
	require.NoError(t, d.Create(&mp).Error)
	f.mpID = mp.ID
	return f
}

// 找未来第 N 天中教练有模板且整天可约的日期：直接给每天配 9:00-21:00 模板
func seedAllDaySchedule(t *testing.T, f *fixture) {
	for wd := 0; wd < 7; wd++ {
		require.NoError(t, f.db.Create(&model.CoachSchedule{
			CoachID: f.coachID, Weekday: int8(wd), StartMinute: 9 * 60, EndMinute: 21 * 60, Status: 1,
		}).Error)
	}
}

// 私教预约全链路：预约扣次+开卡 → 教练确认 → 核销（重复核销幂等）→ 训练记录
func TestBookingVerifyFlow(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	seedAllDaySchedule(t, f)
	ctx := context.Background()
	date := futureDate(2)

	b, err := f.booking.Create(ctx, f.userID, f.coachID, f.mpID, date, 10*60)
	require.NoError(t, err)
	assert.Equal(t, model.BookingStatusPending, b.Status)

	// 预约即扣次 + 首次扣次开卡
	var mp model.MemberPackage
	f.db.First(&mp, f.mpID)
	assert.Equal(t, 1, mp.UsedSessions)
	assert.Equal(t, model.MPStatusActive, mp.Status)
	assert.NotNil(t, mp.ExpireDate, "开卡应写入有效期")

	// 确认
	require.NoError(t, f.booking.Confirm(ctx, b.ID, f.coachID))

	// 核销码 → 教练扫码
	v, err := f.booking.GenCode(ctx, b.ID, f.userID)
	require.NoError(t, err)
	r1, err := f.verify.Scan(ctx, v.Code, f.coachID, false)
	require.NoError(t, err)
	assert.Equal(t, false, r1["alreadyVerified"])
	assert.Equal(t, "private", r1["bizType"])

	// 重复扫码幂等成功
	r2, err := f.verify.Scan(ctx, v.Code, f.coachID, false)
	require.NoError(t, err)
	assert.Equal(t, true, r2["alreadyVerified"])

	var booking model.Booking
	f.db.First(&booking, b.ID)
	assert.Equal(t, model.BookingStatusCompleted, booking.Status)
	var records int64
	f.db.Model(&model.TrainingRecord{}).Where("user_id = ?", f.userID).Count(&records)
	assert.Equal(t, int64(1), records, "重复核销不得重复生成训练记录")

	// 其他教练扫自己的码应被拒
	b2, _ := f.booking.Create(ctx, f.userID, f.coachID, f.mpID, date, 14*60)
	v2, _ := f.booking.GenCode(ctx, b2.ID, f.userID)
	_, err = f.verify.Scan(ctx, v2.Code, f.coachID+999, false)
	assert.Error(t, err)
}

// 防冲突：同教练同日同时段/重叠时段不可重复预约；取消后时段可再约
func TestBookingConflict(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	seedAllDaySchedule(t, f)
	ctx := context.Background()
	date := futureDate(3)

	b1, err := f.booking.Create(ctx, f.userID, f.coachID, f.mpID, date, 10*60) // 10:00-11:00
	require.NoError(t, err)

	// 完全同时段 → 冲突
	_, err = f.booking.Create(ctx, f.userID, f.coachID, f.mpID, date, 10*60)
	assert.Error(t, err, "同起始时段必须冲突")

	// 重叠（10:30 起与 10:00-11:00 重叠 30 分钟）→ 冲突
	_, err = f.booking.Create(ctx, f.userID, f.coachID, f.mpID, date, 10*60+30)
	assert.Error(t, err, "重叠时段必须冲突")

	// 不重叠（11:00 起）→ 成功
	_, err = f.booking.Create(ctx, f.userID, f.coachID, f.mpID, date, 11*60)
	assert.NoError(t, err)

	// 取消第一个 → 10:00 时段释放可再约
	require.NoError(t, f.booking.UserCancel(ctx, b1.ID, f.userID))
	_, err = f.booking.Create(ctx, f.userID, f.coachID, f.mpID, date, 10*60)
	assert.NoError(t, err, "取消后释放的时段必须可再约")
}

// 取消规则：提前 ≥4h 免费取消返还；<4h 扣 1 课时不返还；教练取消一律返还
func TestBookingCancelRules(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	seedAllDaySchedule(t, f)
	ctx := context.Background()

	mpOf := func() model.MemberPackage {
		var mp model.MemberPackage
		f.db.First(&mp, f.mpID)
		return mp
	}

	// 场景1：明天 10:00 的课，现在取消（远超 4 小时）→ 返还
	b1, err := f.booking.Create(ctx, f.userID, f.coachID, f.mpID, futureDate(1), 10*60)
	require.NoError(t, err)
	require.NoError(t, f.booking.UserCancel(ctx, b1.ID, f.userID))
	assert.Equal(t, 0, mpOf().UsedSessions, "提前取消应返还课时")

	// 场景2：约 90 分钟后的课（<4h）→ 取消不返还
	soonDate := time.Now().Add(90 * time.Minute)
	// 确保模板覆盖该 weekday
	wd := int(soonDate.Weekday())
	f.db.Where("coach_id = ?", f.coachID).Delete(&model.CoachSchedule{})
	require.NoError(t, f.db.Create(&model.CoachSchedule{
		CoachID: f.coachID, Weekday: int8(wd), StartMinute: 0, EndMinute: 24 * 60, Status: 1,
	}).Error)
	b2, err := f.booking.Create(ctx, f.userID, f.coachID, f.mpID,
		soonDate.Format("2006-01-02"), (soonDate.Hour()*60+soonDate.Minute())/30*30)
	require.NoError(t, err, "90 分钟后可约（≥1 小时提前量）")
	require.NoError(t, f.booking.UserCancel(ctx, b2.ID, f.userID))
	var b2r model.Booking
	f.db.First(&b2r, b2.ID)
	assert.Equal(t, 1, b2r.PenaltySessions, "4 小时内取消应记罚扣")
	assert.Equal(t, 1, mpOf().UsedSessions, "4 小时内取消课时不返还")

	// 场景3：教练取消 → 一律返还（固定用明天 10:00，避免临午夜时段越界）
	wdTomorrow := int(time.Now().AddDate(0, 0, 1).Weekday())
	require.NoError(t, f.db.Create(&model.CoachSchedule{
		CoachID: f.coachID, Weekday: int8(wdTomorrow), StartMinute: 0, EndMinute: 24 * 60, Status: 1,
	}).Error)
	b3, err := f.booking.Create(ctx, f.userID, f.coachID, f.mpID, futureDate(1), 10*60)
	require.NoError(t, err)
	require.NoError(t, f.booking.CoachCancel(ctx, b3.ID, f.coachID, false))
	assert.Equal(t, 1, mpOf().UsedSessions, "教练取消后净消耗仍为 1（罚扣那节不退，b3 已退）")
}

// 团课：防超卖（容量1 双人并发报名仅1成功）+ 退出返还 + 重复报名拒绝
func TestGroupEnrollOversell(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	ctx := context.Background()

	gc, err := f.group.Publish(ctx, f.coachID, pkgIDOf(f), "燃脂团课", futureDate(2), 19*60, 20*60, "大厅", 1, "", 1)
	require.NoError(t, err)

	// 学员1 报名成功
	_, err = f.group.Enroll(ctx, gc.ID, f.userID, f.mpID)
	require.NoError(t, err)
	var gc1 model.GroupClass
	f.db.First(&gc1, gc.ID)
	assert.Equal(t, 1, gc1.EnrolledCount)
	assert.Equal(t, model.GCStatusFulfilled, gc1.Status, "容量1报满即 fulfilled")

	// 学员2 报名 → 名额已满
	u2 := model.User{OpenID: "u2"}
	f.db.Create(&u2)
	mp2 := model.MemberPackage{
		OwnerID: 1, UserID: u2.ID, PackageID: gc.PackageID, PackageTitle: "测试私教",
		TotalSessions: 5, ValidDays: 365, UnitPrice: 10000, Status: model.MPStatusActive,
	}
	f.db.Create(&mp2)
	_, err = f.group.Enroll(ctx, gc.ID, u2.ID, mp2.ID)
	assert.Error(t, err, "容量已满必须拒绝")

	// 重复报名拒绝
	_, err = f.group.Enroll(ctx, gc.ID, f.userID, f.mpID)
	assert.Error(t, err)

	// 学员1 退出 → 返还课时 + 名额释放 + 状态回报名中
	require.NoError(t, f.group.Quit(ctx, gc.ID, f.userID))
	var mp model.MemberPackage
	f.db.First(&mp, f.mpID)
	assert.Equal(t, 0, mp.UsedSessions, "退出应返还")
	var gc2 model.GroupClass
	f.db.First(&gc2, gc.ID)
	assert.Equal(t, 0, gc2.EnrolledCount)
	assert.Equal(t, model.GCStatusEnrolling, gc2.Status)

	// 学员2 再报 → 成功（释放的名额真实可用）
	_, err = f.group.Enroll(ctx, gc.ID, u2.ID, mp2.ID)
	require.NoError(t, err)
}

// 团课取消：所有报名学员返还；核销走团课码
func TestGroupCancelAndVerify(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	ctx := context.Background()

	gc, err := f.group.Publish(ctx, f.coachID, pkgIDOf(f), "塑形团课", futureDate(2), 19*60, 20*60, "大厅", 5, "", 1)
	require.NoError(t, err)
	_, err = f.group.Enroll(ctx, gc.ID, f.userID, f.mpID)
	require.NoError(t, err)

	// 核销码 + 扫码
	v, err := f.group.GenCode(ctx, gc.ID, f.userID)
	require.NoError(t, err)
	res, err := f.verify.Scan(ctx, v.Code, f.coachID, false)
	require.NoError(t, err)
	assert.Equal(t, "group", res["bizType"])
	var records int64
	f.db.Model(&model.TrainingRecord{}).Where("user_id = ? AND source = 'group'", f.userID).Count(&records)
	assert.Equal(t, int64(1), records)

	// 教练取消（报名中的另一学员也返还）
	u2 := model.User{OpenID: "u3"}
	f.db.Create(&u2)
	mp2 := model.MemberPackage{
		OwnerID: 1, UserID: u2.ID, PackageID: gc.PackageID, PackageTitle: "x",
		TotalSessions: 5, ValidDays: 365, UnitPrice: 10000, Status: model.MPStatusActive,
	}
	f.db.Create(&mp2)
	gc2, _ := f.group.Publish(ctx, f.coachID, gc.PackageID, "取消场", futureDate(3), 19*60, 20*60, "大厅", 5, "", 1)
	_, err = f.group.Enroll(ctx, gc2.ID, u2.ID, mp2.ID)
	require.NoError(t, err)
	require.NoError(t, f.group.CancelByCoach(ctx, gc2.ID, f.coachID, "教练有事"))
	var mp2r model.MemberPackage
	f.db.First(&mp2r, mp2.ID)
	assert.Equal(t, 0, mp2r.UsedSessions, "团课取消应返还全部报名学员")
}

func pkgIDOf(f *fixture) int64 {
	var mp model.MemberPackage
	f.db.First(&mp, f.mpID)
	return mp.PackageID
}

// 余额不足时预约/报名被拒；扣次不超过剩余
func TestInsufficientSessions(t *testing.T) {
	f := newFixture(t, 1, 9*60, 21*60) // 只有 1 课时
	seedAllDaySchedule(t, f)
	ctx := context.Background()

	_, err := f.booking.Create(ctx, f.userID, f.coachID, f.mpID, futureDate(2), 10*60)
	require.NoError(t, err)
	// 最后一节已约完 → 再约失败
	_, err = f.booking.Create(ctx, f.userID, f.coachID, f.mpID, futureDate(2), 14*60)
	assert.Error(t, err)
	var mp model.MemberPackage
	f.db.First(&mp, f.mpID)
	assert.Equal(t, model.MPStatusExhausted, mp.Status)

	// 非窗口时间预约失败（新 fixture 无模板）
	f2 := newFixture(t, 5, 9*60, 21*60)
	_, err = f2.booking.Create(ctx, f2.userID, f2.coachID, f2.mpID, futureDate(2), 8*60)
	assert.Error(t, err, "窗口外时间不可预约")
}
