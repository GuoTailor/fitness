// Package task 内置定时任务：评价邀请（核销 N 天后未评价）、
// 课时不足与有效期预警。P0 单进程 goroutine + 每小时 ticker，
// 不引入外部队列；防重发由 notify_log 唯一键兜底，扫描幂等。
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
}

func NewRunner(db *gorm.DB, mp wechat.MiniProgram, notifyCfg config.NotifyConfig) *Runner {
	cfg := service.NewConfigService(db)
	notify := service.NewNotifyService(db, mp, notifyCfg.Mock, notifyCfg.Templates)
	return &Runner{db: db, cfg: cfg, notify: notify}
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
	log.Printf("[task] 后台任务已启动：评价邀请 / 课时与有效期预警（每小时扫描）")
}

// RunOnce 扫一轮（供测试与手动触发）。
func (r *Runner) RunOnce(ctx context.Context) {
	if n, err := r.ReviewInvites(ctx); err != nil {
		log.Printf("[task] 评价邀请扫描失败: %v", err)
	} else if n > 0 {
		log.Printf("[task] 评价邀请推送 %d 条", n)
	}
	if n, err := r.MemberWarnings(ctx); err != nil {
		log.Printf("[task] 课时/有效期预警扫描失败: %v", err)
	} else if n > 0 {
		log.Printf("[task] 课时/有效期预警推送 %d 条", n)
	}
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
