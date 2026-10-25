package service

// P1 二期 · 批次③ 单测：教练课后反馈 / 体重记录 / 训练周报

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fitness/internal/model"
)

func weekMonday() time.Time {
	n := time.Now()
	return n.AddDate(0, 0, -int(n.Weekday()))
}

// 课后反馈：核销后教练可写、可修订；他人核销 404；学员可见列表与预约详情关联
func TestCoachFeedback(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	seedAllDaySchedule(t, f)
	ctx := context.Background()
	require.NoError(t, f.db.AutoMigrate(&model.ClassFeedback{}, &model.BodyMetric{}))
	fbSvc := NewFeedbackService(f.db)

	// 预约 → 核销
	b, err := f.booking.Create(ctx, f.userID, f.coachID, f.mpID, futureDate(2), 10*60)
	require.NoError(t, err)
	v, err := f.booking.GenCode(ctx, b.ID, f.userID)
	require.NoError(t, err)

	// 未核销不可写
	_, err = fbSvc.Write(ctx, f.coachID, v.ID, "太早了")
	assert.Error(t, err)
	_, err = f.verify.Scan(ctx, v.Code, f.coachID, false)
	require.NoError(t, err)

	// 别的教练不可写（隔离）
	_, err = fbSvc.Write(ctx, f.coachID+999, v.ID, "越权")
	assert.Error(t, err)

	// 本教练写入 → 修订
	fb, err := fbSvc.Write(ctx, f.coachID, v.ID, "深蹲动作标准度不错，下次加强核心")
	require.NoError(t, err)
	fb2, err := fbSvc.Write(ctx, f.coachID, v.ID, "修订：深蹲标准，核心与柔韧性待加强")
	require.NoError(t, err)
	assert.Equal(t, fb.ID, fb2.ID, "同一核销单修订不重复建行")

	// 学员视角：列表 + 预约详情关联
	mine, err := fbSvc.Mine(ctx, f.userID)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Contains(t, mine[0]["feedback"].(model.ClassFeedback).Content, "修订")
	detail := fbSvc.OfBooking(ctx, f.userID, b.ID)
	require.NotNil(t, detail)
	assert.Contains(t, detail["content"], "修订")

	// 他人预约查不到反馈（隔离）
	assert.Nil(t, fbSvc.OfBooking(ctx, f.userID+999, b.ID))
}

// 体重记录：同日 upsert、区间校验、升序输出
func TestBodyMetric(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	ctx := context.Background()
	require.NoError(t, f.db.AutoMigrate(&model.ClassFeedback{}, &model.BodyMetric{}))
	body := NewBodyService(f.db)

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	_, err := body.AddMetric(ctx, f.userID, yesterday, 70000)
	require.NoError(t, err)
	m2, err := body.AddMetric(ctx, f.userID, today, 69500)
	require.NoError(t, err)
	assert.Equal(t, 69500, m2.WeightG)

	// 同日覆盖
	m3, err := body.AddMetric(ctx, f.userID, today, 69000)
	require.NoError(t, err)
	assert.Equal(t, m2.ID, m3.ID, "同日 upsert 复用行")
	assert.Equal(t, 69000, m3.WeightG)

	// 区间与格式拒绝
	_, err = body.AddMetric(ctx, f.userID, today, 5000)
	assert.Error(t, err)
	_, err = body.AddMetric(ctx, f.userID, "bad", 70000)
	assert.Error(t, err)

	rows, err := body.ListMetrics(ctx, f.userID, 90)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.LessOrEqual(t, rows[0].RecordDate, rows[1].RecordDate, "升序输出")
}

// 训练周报：本周/上周聚合、私教团课占比、教练 Top、体重首末差
func TestWeeklyReport(t *testing.T) {
	f := newFixture(t, 10, 9*60, 21*60)
	ctx := context.Background()
	require.NoError(t, f.db.AutoMigrate(&model.ClassFeedback{}, &model.BodyMetric{}))
	body := NewBodyService(f.db)

	mon := weekMonday()
	// 本周：私教 2 + 团课 1；上周：私教 1
	seed := func(offset int, source string) {
		date := mon.AddDate(0, 0, offset).Format("2006-01-02")
		// 落在过去的日期才有效数据意义；未来日期同样计入周报（record_date 无时间校验）
		require.NoError(t, f.db.Create(&model.TrainingRecord{
			UserID: f.userID, CoachID: f.coachID, PackageID: 1,
			RecordDate: date, Project: "测试课", DurationMinutes: 60, Source: source,
		}).Error)
	}
	seed(0, "private")
	seed(1, "private")
	seed(2, "group")
	seed(-7, "private")
	// 体重：周一 70.0kg → 周三 69.2kg
	_, err := body.AddMetric(ctx, f.userID, mon.Format("2006-01-02"), 70000)
	require.NoError(t, err)
	_, err = body.AddMetric(ctx, f.userID, mon.AddDate(0, 0, 2).Format("2006-01-02"), 69200)
	require.NoError(t, err)

	rep, err := f.verify.WeeklyReport(ctx, f.userID, "", body)
	require.NoError(t, err)
	assert.Equal(t, mon.Format("2006-01-02"), rep["weekStart"])
	cur := rep["current"].(map[string]interface{})
	assert.Equal(t, 3, cur["sessions"])
	assert.Equal(t, 180, cur["minutes"])
	assert.Equal(t, 2, cur["private"])
	assert.Equal(t, 1, cur["group"])
	assert.True(t, cur["activeDays"].(int) >= 1)
	prev := rep["prev"].(map[string]interface{})
	assert.Equal(t, 1, prev["sessions"], fmt.Sprint(prev))
	byCoach := rep["byCoach"].([]map[string]interface{})
	require.Len(t, byCoach, 1)
	assert.Equal(t, 3, byCoach[0]["sessions"])
	weight := rep["weight"].(map[string]interface{})
	assert.Equal(t, 70000, weight["first"])
	assert.Equal(t, 69200, weight["last"])
	assert.InDelta(t, -0.8, weight["deltaKg"], 0.0001)
}
