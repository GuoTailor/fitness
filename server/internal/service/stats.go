package service

import (
	"context"
	"time"

	"gorm.io/gorm"

	"fitness/internal/model"
)

// StatsService 管理后台数据看板。
// 口径：GMV = 已支付订单（含退款中/已退款）按实付金额，净额再减退款额；
// 课消率 = 非退款课包 used/total；复购率 = 支付 ≥2 单用户 / 支付 ≥1 单用户。
// P0 数据量小，全量聚合即可，无预聚合表。
type StatsService struct{ db *gorm.DB }

func NewStatsService(db *gorm.DB) *StatsService { return &StatsService{db: db} }

var paidOrderStatuses = []string{model.OrderStatusPaid, model.OrderStatusRefund, model.OrderStatusRefunded}

func (s *StatsService) Dashboard(ctx context.Context) (map[string]interface{}, error) {
	now := time.Now()
	today := now.Format("2006-01-02")
	monthStart := now.Format("2006-01") + "-01"

	// ---- 卡片指标 ----
	var todayGmv, todayCnt int64
	s.db.WithContext(ctx).Model(&model.Order{}).
		Where("status IN ? AND DATE(created_at) = ?", paidOrderStatuses, today).
		Select("COALESCE(SUM(amount),0), COUNT(*)").Row().Scan(&todayGmv, &todayCnt)

	var monthGmv, monthRefund, monthOrders int64
	s.db.WithContext(ctx).Model(&model.Order{}).
		Where("status IN ? AND DATE(created_at) >= ?", paidOrderStatuses, monthStart).
		Select("COALESCE(SUM(amount),0), COALESCE(SUM(refund_amount),0), COUNT(*)").
		Row().Scan(&monthGmv, &monthRefund, &monthOrders)

	var monthLessons, monthMinutes int64
	s.db.WithContext(ctx).Model(&model.TrainingRecord{}).
		Where("record_date >= ?", monthStart).
		Select("COUNT(*), COALESCE(SUM(duration_minutes),0)").Row().Scan(&monthLessons, &monthMinutes)

	var totalUsers int64
	s.db.WithContext(ctx).Model(&model.User{}).Count(&totalUsers)

	var usedSessions, totalSessions int64
	s.db.WithContext(ctx).Model(&model.MemberPackage{}).
		Where("status <> ?", model.MPStatusRefunded).
		Select("COALESCE(SUM(used_sessions),0), COALESCE(SUM(total_sessions),0)").
		Row().Scan(&usedSessions, &totalSessions)

	var buyers, repeaters int64
	var rows []struct{ Cnt int64 }
	s.db.WithContext(ctx).Model(&model.Order{}).
		Where("status IN ?", paidOrderStatuses).
		Select("COUNT(*) as cnt").Group("user_id").Limit(10000).Scan(&rows)
	for _, r := range rows {
		buyers++
		if r.Cnt >= 2 {
			repeaters++
		}
	}

	var pendingReviews, pendingRefunds int64
	s.db.WithContext(ctx).Model(&model.Review{}).
		Where("status = ?", model.ReviewStatusPending).Count(&pendingReviews)
	s.db.WithContext(ctx).Model(&model.Refund{}).
		Where("status = ?", model.RefundStatusPending).Count(&pendingRefunds)

	burnRate := float64(0)
	if totalSessions > 0 {
		burnRate = float64(usedSessions) / float64(totalSessions)
	}
	repurchaseRate := float64(0)
	if buyers > 0 {
		repurchaseRate = float64(repeaters) / float64(buyers)
	}

	// ---- 近 30 天趋势（GMV + 课时消耗）----
	type dayRow struct {
		D string
		V int64
	}
	var gmvRows, lessonRows []dayRow
	s.db.WithContext(ctx).Model(&model.Order{}).
		Where("status IN ? AND DATE(created_at) >= ?", paidOrderStatuses, daysAgo(29)).
		Select("DATE(created_at) as d, COALESCE(SUM(amount),0) as v").
		Group("d").Order("d").Scan(&gmvRows)
	s.db.WithContext(ctx).Model(&model.TrainingRecord{}).
		Where("record_date >= ?", daysAgo(29)).
		Select("record_date as d, COUNT(*) as v").
		Group("d").Order("d").Scan(&lessonRows)
	gmvByDay := map[string]int64{}
	for _, r := range gmvRows {
		gmvByDay[dateOnly(r.D)] = r.V
	}
	lessonByDay := map[string]int64{}
	for _, r := range lessonRows {
		lessonByDay[dateOnly(r.D)] = r.V
	}
	dates := make([]string, 0, 30)
	gmvSeries := make([]int64, 0, 30)
	lessonSeries := make([]int64, 0, 30)
	for i := 29; i >= 0; i-- {
		d := daysAgo(i)
		dates = append(dates, d)
		gmvSeries = append(gmvSeries, gmvByDay[d])
		lessonSeries = append(lessonSeries, lessonByDay[d])
	}

	// ---- 教练本月排行（课时数）----
	var rankRows []struct {
		CoachID int64
		Cnt     int64
	}
	s.db.WithContext(ctx).Model(&model.TrainingRecord{}).
		Where("record_date >= ?", monthStart).
		Select("coach_id, COUNT(*) as cnt").
		Group("coach_id").Order("cnt DESC").Limit(5).Scan(&rankRows)
	coachRank := make([]map[string]interface{}, 0, len(rankRows))
	for _, r := range rankRows {
		item := map[string]interface{}{"coachId": r.CoachID, "lessons": r.Cnt, "name": "已删除教练"}
		var c model.Coach
		if err := s.db.WithContext(ctx).First(&c, r.CoachID).Error; err == nil {
			item["name"] = c.Name
			item["ratingAvg"] = c.RatingAvg
			item["ratingCount"] = c.RatingCount
		}
		coachRank = append(coachRank, item)
	}

	return map[string]interface{}{
		"todayGmv": todayGmv, "monthGmv": monthGmv, "monthNetGmv": monthGmv - monthRefund,
		"monthOrders": monthOrders, "monthLessons": monthLessons, "monthMinutes": monthMinutes,
		"totalUsers": totalUsers, "burnRate": burnRate, "repurchaseRate": repurchaseRate,
		"pendingReviews": pendingReviews, "pendingRefunds": pendingRefunds,
		"trend": map[string]interface{}{
			"dates": dates, "gmv": gmvSeries, "lessons": lessonSeries,
		},
		"coachRank": coachRank,
	}, nil
}

func daysAgo(n int) string {
	return time.Now().AddDate(0, 0, -n).Format("2006-01-02")
}
