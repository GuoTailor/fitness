// Package task 内置定时任务。两类职责：
// 1) 消息触达：评价邀请、课时/有效期预警（notify_log 防重发）；
// 2) 状态推进（超时/反向链路）：超时关单、课包过期、预约过期（no-show 罚没）、
//    团课结课、退款单轮询推进终态——这些状态此前只定义、无人写入。
// P0 单进程 goroutine + 每小时 ticker，不引入外部队列；所有推进均为幂等条件更新。
package task

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"fitness/internal/config"
	"fitness/internal/model"
	"fitness/internal/service"
	"fitness/internal/wechat"
)

type Runner struct {
	db     *gorm.DB
	cfg    *service.ConfigService
	notify *service.NotifyService
	orders *service.OrderService
}

func NewRunner(db *gorm.DB, mp wechat.MiniProgram, notifyCfg config.NotifyConfig, pay wechat.PaymentProvider) *Runner {
	cfg := service.NewConfigService(db)
	return &Runner{
		db:     db,
		cfg:    cfg,
		notify: service.NewNotifyService(db, mp, notifyCfg.Mock, notifyCfg.Templates),
		orders: service.NewOrderService(db, cfg, pay),
	}
}

// Start 启动后台循环（启动即扫一遍，之后每小时一次）。
func (r *Runner) Start(ctx context.Context) {
	go func() {
		r.RunOnce(ctx)
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.RunOnce(ctx)
			}
		}
	}()
	log.Printf("[task] 后台任务已启动：评价邀请/课时预警 + 关单/过期/结课/退款轮询（每小时扫描）")
}

// RunOnce 扫一轮（供测试与手动触发）。
func (r *Runner) RunOnce(ctx context.Context) {
	run := func(name string, fn func() (int64, error)) {
		n, err := fn()
		if err != nil {
			log.Printf("[task] %s失败: %v", name, err)
		} else if n > 0 {
			log.Printf("[task] %s处理 %d 条", name, n)
		}
	}
	run("评价邀请", func() (int64, error) {
		n, err := r.ReviewInvites(ctx)
		return int64(n), err
	})
	run("课时/有效期预警", func() (int64, error) {
		n, err := r.MemberWarnings(ctx)
		return int64(n), err
	})
	run("超时关单", func() (int64, error) { return r.CloseExpiredOrders(ctx) })
	run("课包过期", func() (int64, error) { return r.ExpireMemberPackages(ctx) })
	run("预约过期", func() (int64, error) { return r.ExpireBookings(ctx) })
	run("团课结课", func() (int64, error) { return r.FinishGroupClasses(ctx) })
	run("优惠券过期", func() (int64, error) { return r.ExpireUserCoupons(ctx) })
	run("退款轮询", func() (int64, error) {
		n, err := r.orders.PollApprovedRefunds(ctx)
		return int64(n), err
	})
}

// ReviewInvites 核销满 N 天且尚未评价 → 推评价邀请（notify_log 防重发，历史数据首次补推一次）。
func (r *Runner) ReviewInvites(ctx context.Context) (int, error) {
	days := r.cfg.GetInt(ctx, model.CfgReviewInviteAfterDays, 1)
	if days < 0 {
		days = 1
	}
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	var vs []model.Verification
	err := r.db.WithContext(ctx).Raw(
		"SELECT v.* FROM verification v LEFT JOIN review r ON r.verification_id = v.id "+
			"WHERE v.status = ? AND DATE(v.verified_at) <= ? AND r.id IS NULL LIMIT 200",
		model.VerifyStatusVerified, cutoff).Scan(&vs).Error
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, v := range vs {
		ok, err := r.notify.Send(ctx, v.UserID, model.NotifySceneReviewInvite, v.ID,
			map[string]string{"thing1": "您有一节已完成的课程待评价"})
		if err != nil {
			log.Printf("[task] 评价邀请 user=%d verification=%d 失败: %v", v.UserID, v.ID, err)
			continue
		}
		if ok {
			sent++
		}
	}
	return sent, nil
}

// MemberWarnings 生效中课包：剩余课时 ≤N 节、距到期 ≤N 天 → 预警（每个课包每场景只推一次）。
func (r *Runner) MemberWarnings(ctx context.Context) (int, error) {
	sent := 0
	today := time.Now().Format("2006-01-02")

	if warnSessions := r.cfg.GetInt(ctx, model.CfgMemberWarnSessions, 3); warnSessions > 0 {
		var mps []model.MemberPackage
		if err := r.db.WithContext(ctx).
			Where("status = ? AND (total_sessions - used_sessions) <= ?",
				model.MPStatusActive, warnSessions).
			Limit(200).Find(&mps).Error; err != nil {
			return sent, err
		}
		for _, mp := range mps {
			ok, err := r.notify.Send(ctx, mp.UserID, model.NotifySceneSessionsWarning, mp.ID,
				map[string]string{"thing1": fmt.Sprintf("课时仅剩 %d 节，及时续课哦", mp.RemainingSessions())})
			if err != nil {
				log.Printf("[task] 课时预警 mp=%d 失败: %v", mp.ID, err)
				continue
			}
			if ok {
				sent++
			}
		}
	}

	if warnDays := r.cfg.GetInt(ctx, model.CfgMemberWarnExpireDays, 14); warnDays > 0 {
		deadline := time.Now().AddDate(0, 0, warnDays).Format("2006-01-02")
		var mps []model.MemberPackage
		if err := r.db.WithContext(ctx).
			Where("status = ? AND expire_date IS NOT NULL AND expire_date > ? AND expire_date <= ?",
				model.MPStatusActive, today, deadline).
			Limit(200).Find(&mps).Error; err != nil {
			return sent, err
		}
		for _, mp := range mps {
			expire := ""
			if mp.ExpireDate != nil {
				expire = mp.ExpireDate.Format("2006-01-02")
			}
			ok, err := r.notify.Send(ctx, mp.UserID, model.NotifySceneExpireWarning, mp.ID,
				map[string]string{"thing1": "课包 " + expire + " 到期，剩余课时请尽快使用"})
			if err != nil {
				log.Printf("[task] 到期预警 mp=%d 失败: %v", mp.ID, err)
				continue
			}
			if ok {
				sent++
			}
		}
	}
	return sent, nil
}

// CloseExpiredOrders 超时关单（M2）：挂单 2 小时未支付关闭，旧价订单不可无限期支付；
// 关单后的迟到回调在 HandlePaySuccess 内幂等应答。
func (r *Runner) CloseExpiredOrders(ctx context.Context) (int64, error) {
	cutoff := time.Now().Add(-2 * time.Hour)
	res := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("status = ? AND created_at < ?", model.OrderStatusPending, cutoff).
		Update("status", model.OrderStatusClosed)
	return res.RowsAffected, res.Error
}

// ExpireMemberPackages 课包过期（M3）：生效中且有效期已过 → expired（消费入口另有硬校验兜底）。
func (r *Runner) ExpireMemberPackages(ctx context.Context) (int64, error) {
	today := time.Now().Format("2006-01-02")
	res := r.db.WithContext(ctx).Model(&model.MemberPackage{}).
		Where("status = ? AND expire_date IS NOT NULL AND expire_date < ?", model.MPStatusActive, today).
		Update("status", model.MPStatusExpired)
	return res.RowsAffected, res.Error
}

// ExpireBookings 预约过期（no-show）：开课时间已过仍未核销 → expired，
// 释放时段、作废核销码；课时已在预约时扣除，不返还（penalty_sessions=1 标记罚没）。
func (r *Runner) ExpireBookings(ctx context.Context) (int64, error) {
	now := time.Now()
	today := now.Format("2006-01-02")
	nowMinute := now.Hour()*60 + now.Minute()
	var bookings []model.Booking
	err := r.db.WithContext(ctx).
		Where("status IN ? AND (booking_date < ? OR (booking_date = ? AND end_minute <= ?))",
			[]string{model.BookingStatusPending, model.BookingStatusConfirmed}, today, today, nowMinute).
		Limit(200).Find(&bookings).Error
	if err != nil {
		return 0, err
	}
	done := int64(0)
	for _, b := range bookings {
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&model.Booking{}).
				Where("id = ? AND status IN ?", b.ID,
					[]string{model.BookingStatusPending, model.BookingStatusConfirmed}).
				Updates(map[string]interface{}{
					"status": model.BookingStatusExpired, "penalty_sessions": 1,
				})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return nil // 并发已处理
			}
			if err := tx.Model(&model.Booking{}).Where("id = ?", b.ID).
				Update("slot_token", model.ReleasedSlotToken(b.ID)).Error; err != nil {
				return err
			}
			return tx.Model(&model.Verification{}).
				Where("booking_id = ? AND status = ?", b.ID, model.VerifyStatusPending).
				Update("status", model.VerifyStatusExpired).Error
		})
		if err != nil {
			log.Printf("[task] 预约 %d 过期处理失败: %v", b.ID, err)
			continue
		}
		done++
	}
	return done, nil
}

// FinishGroupClasses 团课结课：开课时间已过且未取消 → finished（此前永远挂在报名中/满员）。
func (r *Runner) FinishGroupClasses(ctx context.Context) (int64, error) {
	now := time.Now()
	today := now.Format("2006-01-02")
	nowMinute := now.Hour()*60 + now.Minute()
	res := r.db.WithContext(ctx).Model(&model.GroupClass{}).
		Where("status IN ? AND (class_date < ? OR (class_date = ? AND end_minute <= ?))",
			[]string{model.GCStatusEnrolling, model.GCStatusFulfilled, model.GCStatusClosed},
			today, today, nowMinute).
		Update("status", model.GCStatusFinished)
	return res.RowsAffected, res.Error
}

// ExpireUserCoupons 可用券过期：expire_at 已过 → expired（读路径有懒更新，此处全量兜底）
func (r *Runner) ExpireUserCoupons(ctx context.Context) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.UserCoupon{}).
		Where("status = ? AND expire_at <= ?", model.CouponUsable, time.Now()).
		Update("status", model.CouponExpired)
	return res.RowsAffected, res.Error
}
