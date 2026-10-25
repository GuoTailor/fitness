package service

// P1 二期 · 批次② 单测：教练请假屏蔽 / 团课候补自动递补

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fitness/internal/model"
)

// 请假：SlotsOfDate 排除重叠格、Create 拒绝、撤销后恢复、受影响预约计数
func TestCoachLeave(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	seedAllDaySchedule(t, f)
	ctx := context.Background()
	date := futureDate(3)

	leave, affected, err := f.booking.AddLeave(ctx, f.coachID, date, 10*60, 12*60, "休假")
	require.NoError(t, err)
	assert.Equal(t, 0, affected)

	slots, err := f.booking.SlotsOfDate(ctx, f.coachID, date)
	require.NoError(t, err)
	has := map[int]bool{}
	for _, s := range slots {
		has[s.StartMinute] = true
	}
	assert.True(t, has[9*60], "9:00 可约")
	assert.True(t, has[12*60], "12:00 可约")
	for _, m := range []int{9*60 + 30, 10 * 60, 10*60 + 30, 11 * 60, 11*60 + 30} {
		assert.False(t, has[m], "与请假重叠的格应被排除: %d", m)
	}

	// 预约落在请假区间 → 拒绝
	_, err = f.booking.Create(ctx, f.userID, f.coachID, f.mpID, date, 10*60+30)
	assert.ErrorContains(t, err, "请假")

	// 已有预约后再登记请假 → 返回受影响预约数（不自动取消）
	b, err := f.booking.Create(ctx, f.userID, f.coachID, f.mpID, date, 14*60)
	require.NoError(t, err)
	_ = b
	_, affected, err = f.booking.AddLeave(ctx, f.coachID, date, 13*60, 15*60, "临时有事")
	require.NoError(t, err)
	assert.Equal(t, 1, affected)

	// 撤销请假后可再约
	require.NoError(t, f.booking.CancelLeave(ctx, f.coachID, leave.ID))
	_, err = f.booking.Create(ctx, f.userID, f.coachID, f.mpID, date, 16*60)
	require.NoError(t, err)
	// 他人请假记录不可删
	assert.Error(t, f.booking.CancelLeave(ctx, f.coachID+999, leave.ID))
}

// 候补：满员登记 → 退出自动递补（FIFO、扣次、状态 promoted）；
// 递补时课包失效 → failed 跳过；取消候补；教练取消作废候补
func TestGroupWaitlistPromotion(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	ctx := context.Background()

	gc, err := f.group.Publish(ctx, f.coachID, pkgIDOf(f), "候补团课", futureDate(2), 19*60, 20*60, "大厅", 1, "", 1)
	require.NoError(t, err)

	// 学员1 报满唯一名额
	_, err = f.group.Enroll(ctx, gc.ID, f.userID, f.mpID)
	require.NoError(t, err)

	// 学员2/3 候补（2 先到）
	u2 := model.User{OpenID: "wl-u2"}
	u3 := model.User{OpenID: "wl-u3"}
	require.NoError(t, f.db.Create(&u2).Error)
	require.NoError(t, f.db.Create(&u3).Error)
	newMp := func(uid int64, pkgID int64) *model.MemberPackage {
		mp := model.MemberPackage{
			OwnerID: 1, UserID: uid, PackageID: pkgID, PackageTitle: "测试私教",
			TotalSessions: 5, ValidDays: 365, UnitPrice: 10000, Status: model.MPStatusActive,
		}
		require.NoError(t, f.db.Create(&mp).Error)
		return &mp
	}
	mp2 := newMp(u2.ID, gc.PackageID)
	mp3 := newMp(u3.ID, gc.PackageID)

	wl2, err := f.group.Waitlist(ctx, gc.ID, u2.ID, mp2.ID)
	require.NoError(t, err)
	_, err = f.group.Waitlist(ctx, gc.ID, u3.ID, mp3.ID)
	require.NoError(t, err)
	// 重复候补拒绝；已报名者不可候补
	_, err = f.group.Waitlist(ctx, gc.ID, u2.ID, mp2.ID)
	assert.Error(t, err)
	_, err = f.group.Waitlist(ctx, gc.ID, f.userID, f.mpID)
	assert.Error(t, err)

	// Detail 标记 waitlisted
	detail, err := f.group.Detail(ctx, gc.ID, u2.ID)
	require.NoError(t, err)
	assert.Equal(t, true, detail["waitlisted"])

	// 学员1 退出 → FIFO 递补学员2（占位+扣次）
	require.NoError(t, f.group.Quit(ctx, gc.ID, f.userID))
	var wl2After model.GroupWaitlist
	require.NoError(t, f.db.First(&wl2After, wl2.ID).Error)
	assert.Equal(t, model.WLStatusPromoted, wl2After.Status)
	assert.NotNil(t, wl2After.PromotedAt)
	var mp2After model.MemberPackage
	require.NoError(t, f.db.First(&mp2After, mp2.ID).Error)
	assert.Equal(t, 1, mp2After.UsedSessions, "递补即扣次")
	var gcx model.GroupClass
	require.NoError(t, f.db.First(&gcx, gc.ID).Error)
	assert.Equal(t, 1, gcx.EnrolledCount)
	assert.Equal(t, model.GCStatusFulfilled, gcx.Status)
	var wl3After model.GroupWaitlist
	require.NoError(t, f.db.Where("user_id = ?", u3.ID).First(&wl3After).Error)
	assert.Equal(t, model.WLStatusWaiting, wl3After.Status, "未轮到不递补")

	// 学员3 的课包失效（用完）→ 学员2 再退出时跳过 3，名额保持空缺
	require.NoError(t, f.db.Model(&model.MemberPackage{}).Where("id = ?", mp3.ID).
		Update("used_sessions", 5).Error)
	require.NoError(t, f.group.Quit(ctx, gc.ID, u2.ID))
	require.NoError(t, f.db.Where("user_id = ?", u3.ID).First(&wl3After).Error)
	assert.Equal(t, model.WLStatusFailed, wl3After.Status, "课包不可用标 failed")
	require.NoError(t, f.db.First(&gcx, gc.ID).Error)
	assert.Equal(t, 0, gcx.EnrolledCount)
	assert.Equal(t, model.GCStatusEnrolling, gcx.Status, "无人递补名额保持开放")

	// failed 后可重新登记候补（复活为 waiting）
	_, err = f.group.Waitlist(ctx, gc.ID, u3.ID, mp3.ID)
	assert.Error(t, err, "课包仍不可用应被校验拒绝")

	// 教练取消 → 候补作废
	u4 := model.User{OpenID: "wl-u4"}
	require.NoError(t, f.db.Create(&u4).Error)
	mp4 := newMp(u4.ID, gc.PackageID)
	wl4, err := f.group.Waitlist(ctx, gc.ID, u4.ID, mp4.ID)
	require.NoError(t, err)
	require.NoError(t, f.group.CancelByCoach(ctx, gc.ID, f.coachID, "教练有事"))
	var wl4After model.GroupWaitlist
	require.NoError(t, f.db.First(&wl4After, wl4.ID).Error)
	assert.Equal(t, model.WLStatusCanceled, wl4After.Status)
}

// 取消候补：waiting → canceled
func TestGroupWaitlistCancel(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	ctx := context.Background()
	gc, err := f.group.Publish(ctx, f.coachID, pkgIDOf(f), "取消候补团课", futureDate(2), 19*60, 20*60, "大厅", 2, "", 1)
	require.NoError(t, err)

	u2 := model.User{OpenID: "wlc-u2"}
	require.NoError(t, f.db.Create(&u2).Error)
	mp2 := model.MemberPackage{
		OwnerID: 1, UserID: u2.ID, PackageID: gc.PackageID, PackageTitle: "测试私教",
		TotalSessions: 5, ValidDays: 365, UnitPrice: 10000, Status: model.MPStatusActive,
	}
	require.NoError(t, f.db.Create(&mp2).Error)
	_, err = f.group.Waitlist(ctx, gc.ID, u2.ID, mp2.ID)
	require.NoError(t, err)
	require.NoError(t, f.group.CancelWaitlist(ctx, gc.ID, u2.ID))
	assert.Error(t, f.group.CancelWaitlist(ctx, gc.ID, u2.ID), "取消后不可再取消")
}
