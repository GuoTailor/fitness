package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fitness/internal/model"
)

// 评价闭环：核销 → 待评价 → 双维度提交 → 幂等/隔离校验 → 审核通过 →
// 聚合刷新 → 教练回复 → 隐藏后聚合回落
func TestReviewFlow(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	seedAllDaySchedule(t, f)
	require.NoError(t, f.db.AutoMigrate(&model.Review{}, &model.NotifyLog{}))
	ctx := context.Background()

	cfg := NewConfigService(f.db)
	review := NewReviewService(f.db, cfg)

	b, err := f.booking.Create(ctx, f.userID, f.coachID, f.mpID, futureDate(2), 10*60)
	require.NoError(t, err)
	v, err := f.booking.GenCode(ctx, b.ID, f.userID)
	require.NoError(t, err)
	_, err = f.verify.Scan(ctx, v.Code, f.coachID, false)
	require.NoError(t, err)

	// 未核销的预约不可评价
	b2, err := f.booking.Create(ctx, f.userID, f.coachID, f.mpID, futureDate(2), 14*60)
	require.NoError(t, err)
	v2, err := f.booking.GenCode(ctx, b2.ID, f.userID)
	require.NoError(t, err)
	assert.Error(t, review.Submit(ctx, f.userID, v2.ID, 5, 5, "未核销"))

	// 待评价列表：仅已核销未评价的 1 条
	pending, err := review.Reviewable(ctx, f.userID)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, v.ID, pending[0]["verificationId"])
	assert.Equal(t, "测试私教", pending[0]["projectName"])

	// 提交：一次落双维度两条
	require.NoError(t, review.Submit(ctx, f.userID, v.ID, 4, 5, "练得很扎实"))
	var cnt int64
	f.db.Model(&model.Review{}).Count(&cnt)
	assert.EqualValues(t, 2, cnt)

	// 待评价清空；重复提交被 uk_review 拦截
	pending, _ = review.Reviewable(ctx, f.userID)
	assert.Len(t, pending, 0)
	assert.Error(t, review.Submit(ctx, f.userID, v.ID, 4, 5, "再评一次"))

	// 数据隔离：他人核销记录 → 404 语义
	other := model.User{OpenID: "review-other"}
	require.NoError(t, f.db.Create(&other).Error)
	assert.Error(t, review.Submit(ctx, other.ID, v.ID, 5, 5, "冒用"))

	// 非法评分
	assert.Error(t, review.Submit(ctx, f.userID, v.ID, 0, 5, "x"))
	assert.Error(t, review.Submit(ctx, f.userID, v.ID, 5, 6, "x"))

	// 审核前不可见
	items, total, err := review.ListApproved(ctx, model.ReviewTargetCoach, f.coachID, 1, 10)
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.Empty(t, items)

	// 审核通过：公开可见 + 教练评分聚合刷新（4,5 两条中 coach 维度是 5）
	var pkgReview, coachReview model.Review
	require.NoError(t, f.db.Where("target_type = ?", model.ReviewTargetPackage).First(&pkgReview).Error)
	require.NoError(t, f.db.Where("target_type = ?", model.ReviewTargetCoach).First(&coachReview).Error)
	require.NoError(t, review.Audit(ctx, pkgReview.ID, "approve"))
	require.NoError(t, review.Audit(ctx, coachReview.ID, "approve"))

	_, total, err = review.ListApproved(ctx, model.ReviewTargetCoach, f.coachID, 1, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	_, total, err = review.ListApproved(ctx, model.ReviewTargetPackage, pkgReview.PackageID, 1, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)

	var coach model.Coach
	f.db.First(&coach, f.coachID)
	assert.EqualValues(t, 5, coach.RatingAvg)
	assert.EqualValues(t, 1, coach.RatingCount)

	// 教练回复（别人的教练号不可回复）
	require.NoError(t, review.Reply(ctx, f.coachID, coachReview.ID, "感谢反馈！"))
	assert.Error(t, review.Reply(ctx, f.coachID+999, coachReview.ID, "冒充"))
	var replied model.Review
	f.db.First(&replied, coachReview.ID)
	assert.Equal(t, "感谢反馈！", replied.CoachReply)
	assert.NotNil(t, replied.CoachReplyAt)

	// 隐藏后聚合回落、公开列表清空
	require.NoError(t, review.Audit(ctx, coachReview.ID, "hide"))
	f.db.First(&coach, f.coachID)
	assert.EqualValues(t, 0, coach.RatingAvg)
	assert.EqualValues(t, 0, coach.RatingCount)
	_, total, _ = review.ListApproved(ctx, model.ReviewTargetCoach, f.coachID, 1, 10)
	assert.Zero(t, total)

	// 我的评价：两维度都在
	mine, err := review.MyReviews(ctx, f.userID)
	require.NoError(t, err)
	assert.Len(t, mine, 2)

	// 精选开关
	require.NoError(t, review.SetFeatured(ctx, pkgReview.ID, true))
	var featured model.Review
	f.db.First(&featured, pkgReview.ID)
	assert.EqualValues(t, 1, featured.Featured)
}

// 自动过审开关：review.auto_approve=1 时提交即生效并刷新聚合
func TestReviewAutoApprove(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	seedAllDaySchedule(t, f)
	require.NoError(t, f.db.AutoMigrate(&model.Review{}, &model.NotifyLog{}))
	ctx := context.Background()
	require.NoError(t, NewConfigService(f.db).Set(ctx, model.CfgReviewAutoApprove, "1", ""))
	review := NewReviewService(f.db, NewConfigService(f.db))

	b, err := f.booking.Create(ctx, f.userID, f.coachID, f.mpID, futureDate(2), 10*60)
	require.NoError(t, err)
	v, err := f.booking.GenCode(ctx, b.ID, f.userID)
	require.NoError(t, err)
	_, err = f.verify.Scan(ctx, v.Code, f.coachID, false)
	require.NoError(t, err)

	require.NoError(t, review.Submit(ctx, f.userID, v.ID, 5, 5, "免审直出"))
	_, total, err := review.ListApproved(ctx, model.ReviewTargetCoach, f.coachID, 1, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total, "auto_approve=1 提交即通过")
	var coach model.Coach
	f.db.First(&coach, f.coachID)
	assert.EqualValues(t, 1, coach.RatingCount)
}

// 订阅消息防重发：同 (user, scene, biz) 只落一条 notify_log
func TestNotifyDedup(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	require.NoError(t, f.db.AutoMigrate(&model.Review{}, &model.NotifyLog{}))
	ctx := context.Background()
	n := NewNotifyService(f.db, nil, true, nil)

	_, err := n.Send(ctx, f.userID, model.NotifySceneBookingNew, 1, nil)
	require.NoError(t, err)
	_, err = n.Send(ctx, f.userID, model.NotifySceneBookingNew, 1, nil) // 重复 → 幂等
	require.NoError(t, err)
	_, err = n.Send(ctx, f.userID, model.NotifySceneBookingNew, 2, nil) // 不同 biz → 新记录
	require.NoError(t, err)

	var cnt int64
	f.db.Model(&model.NotifyLog{}).Count(&cnt)
	assert.EqualValues(t, 2, cnt)
}

// 预约/确认通知挂点：booking_new→教练账号、booking_confirmed→学员（mock 只验落库）
func TestBookingNotifyHooks(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	seedAllDaySchedule(t, f)
	require.NoError(t, f.db.AutoMigrate(&model.Review{}, &model.NotifyLog{}))
	ctx := context.Background()

	coachUser := model.User{OpenID: "coach-login"}
	require.NoError(t, f.db.Create(&coachUser).Error)
	require.NoError(t, f.db.Model(&model.Coach{}).Where("id = ?", f.coachID).Update("user_id", coachUser.ID).Error)

	notify := NewNotifyService(f.db, nil, true, nil)
	f.booking = NewBookingService(f.db, NewConfigService(f.db)).WithNotify(notify)

	b, err := f.booking.Create(ctx, f.userID, f.coachID, f.mpID, futureDate(2), 10*60)
	require.NoError(t, err)
	require.NoError(t, f.booking.Confirm(ctx, b.ID, f.coachID))

	var logs []model.NotifyLog
	f.db.Order("id").Find(&logs)
	require.Len(t, logs, 2)
	assert.Equal(t, model.NotifySceneBookingNew, logs[0].Scene)
	assert.Equal(t, coachUser.ID, logs[0].UserID)
	assert.Equal(t, model.NotifySceneBookingConfirmed, logs[1].Scene)
	assert.Equal(t, f.userID, logs[1].UserID)
}
