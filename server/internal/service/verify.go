package service

import (
	"context"
	"time"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
)

// VerifyService 扫码核销。课时在预约/报名时已扣，核销只做三件事：
// 翻转核销码状态（幂等闸门）→ 完成预约/报名 → 生成训练记录。
type VerifyService struct{ db *gorm.DB }

func NewVerifyService(db *gorm.DB) *VerifyService { return &VerifyService{db: db} }

// Scan 扫码核销。coachID>0 为教练扫码（校验归属）；byAdmin=true 为后台代核销。
func (s *VerifyService) Scan(ctx context.Context, code string, coachID int64, byAdmin bool) (map[string]interface{}, error) {
	if code == "" {
		return nil, errs.BadParam("核销码不能为空")
	}
	var v model.Verification
	if err := s.db.WithContext(ctx).Where("code = ?", code).First(&v).Error; err != nil {
		return nil, errs.NotFound("核销码不存在")
	}
	if v.Status == model.VerifyStatusVerified {
		return s.scanResult(ctx, &v, true), nil // 重复扫码幂等成功
	}
	if v.Status != model.VerifyStatusPending {
		return nil, errs.Biz("该核销码已失效")
	}
	if coachID > 0 && !byAdmin && v.CoachID != coachID {
		return nil, errs.Forbidden("只能核销自己课程的学员")
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {		// 幂等闸门：pending → verified 只允许一个事务成功
		res := tx.Model(&model.Verification{}).
			Where("id = ? AND status = ?", v.ID, model.VerifyStatusPending).
			Updates(map[string]interface{}{
				"status": model.VerifyStatusVerified, "verified_at": time.Now(),
				"verified_by": verifiedBy(coachID, byAdmin),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // 并发另一请求已核销
		}

		switch v.BizType {
		case "private":
			var b model.Booking
			if err := tx.Where("id = ?", v.BookingID).First(&b).Error; err != nil {
				return errs.NotFound("预约不存在")
			}
			if b.Status != model.BookingStatusPending && b.Status != model.BookingStatusConfirmed {
				return errs.Biz("该预约当前状态不可核销")
			}
			if err := tx.Model(&model.Booking{}).
				Where("id = ? AND status IN ?", b.ID,
					[]string{model.BookingStatusPending, model.BookingStatusConfirmed}).
				Update("status", model.BookingStatusCompleted).Error; err != nil {
				return err
			}
			// 核销即释放时段占用（token 置 done，允许教练同一时段接新预约）
			tx.Model(&model.Booking{}).Where("id = ?", b.ID).
				Update("slot_token", model.ReleasedSlotToken(b.ID))
			var pkg model.CoursePackage
			tx.First(&pkg, b.PackageID)
			return tx.Create(&model.TrainingRecord{
				UserID: v.UserID, CoachID: v.CoachID, PackageID: b.PackageID,
				RecordDate: dateOnly(b.BookingDate), Project: pkg.Title,
				DurationMinutes: b.EndMinute - b.StartMinute, Source: "private",
				Remark: "私教课核销",
			}).Error

		case "group":
			var ge model.GroupEnrollment
			if err := tx.Where("id = ?", v.EnrollmentID).First(&ge).Error; err != nil {
				return errs.NotFound("报名记录不存在")
			}
			if ge.Status != model.GEStatusEnrolled {
				return errs.Biz("该报名当前状态不可核销")
			}
			if err := tx.Model(&model.GroupEnrollment{}).
				Where("id = ? AND status = ?", ge.ID, model.GEStatusEnrolled).
				Update("status", model.GEStatusVerified).Error; err != nil {
				return err
			}
			var gc model.GroupClass
			tx.First(&gc, ge.GroupClassID)
			return tx.Create(&model.TrainingRecord{
				UserID: v.UserID, CoachID: v.CoachID, PackageID: gc.PackageID,
				RecordDate: dateOnly(gc.ClassDate), Project: gc.Title,
				DurationMinutes: gc.EndMinute - gc.StartMinute, Source: "group",
				Remark: "团课核销",
			}).Error
		}
		return errs.BadParam("未知的核销类型")
	})
	if err != nil {
		return nil, err
	}
	return s.scanResult(ctx, &v, false), nil
}

func verifiedBy(coachID int64, byAdmin bool) string {
	if byAdmin {
		return "admin"
	}
	return "coach"
}

func (s *VerifyService) scanResult(ctx context.Context, v *model.Verification, already bool) map[string]interface{} {
	var user model.User
	s.db.WithContext(ctx).First(&user, v.UserID)
	name := user.Nickname
	if name == "" {
		name = "学员"
	}
	return map[string]interface{}{
		"alreadyVerified": already,
		"bizType":         v.BizType,
		"userName":        name,
		"userPhone":       user.Phone,
		"code":            v.Code,
		"verifiedAt":      v.VerifiedAt,
	}
}

// TrainingStats 训练统计（周/月上课数与课时消耗）
func (s *VerifyService) TrainingStats(ctx context.Context, userID int64) (map[string]interface{}, error) {
	now := time.Now()
	monthStart := now.Format("2006-01") + "-01"
	weekStart := now.AddDate(0, 0, -int(now.Weekday())).Format("2006-01-02")
	count := func(from string) map[string]interface{} {
		var lessons int64
		var minutes int64
		s.db.WithContext(ctx).Model(&model.TrainingRecord{}).
			Where("user_id = ? AND record_date >= ?", userID, from).
			Select("COUNT(*), COALESCE(SUM(duration_minutes),0)").Row().Scan(&lessons, &minutes)
		return map[string]interface{}{"lessons": lessons, "minutes": minutes}
	}
	return map[string]interface{}{
		"week": count(weekStart), "month": count(monthStart),
	}, nil
}

// TrainingRecords 训练记录列表
func (s *VerifyService) TrainingRecords(ctx context.Context, userID int64, from, to string) ([]model.TrainingRecord, error) {
	q := s.db.WithContext(ctx).Where("user_id = ?", userID)
	if from != "" {
		q = q.Where("record_date >= ?", from)
	}
	if to != "" {
		q = q.Where("record_date <= ?", to)
	}
	var list []model.TrainingRecord
	err := q.Order("record_date DESC, id DESC").Limit(200).Find(&list).Error
	return list, err
}

// TrainingCalendar 某月训练日历：[{date, count}]，小程序日历打点用。
func (s *VerifyService) TrainingCalendar(ctx context.Context, userID int64, month string) ([]map[string]interface{}, error) {
	if _, err := time.Parse("2006-01", month); err != nil {
		return nil, errs.BadParam("月份格式应为 YYYY-MM")
	}
	var rows []struct {
		RecordDate string
		Cnt        int
	}
	err := s.db.WithContext(ctx).Model(&model.TrainingRecord{}).
		Select("record_date, COUNT(*) as cnt").
		Where("user_id = ? AND record_date LIKE ?", userID, month+"%").
		Group("record_date").Order("record_date").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]interface{}{"date": dateOnly(r.RecordDate), "count": r.Cnt})
	}
	return out, nil
}
