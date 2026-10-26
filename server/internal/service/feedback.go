package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
)

// FeedbackService 教练课后反馈（规划 §4 学员记录）。
// 教练对本人核销单写训练反馈，学员在训练跟踪/预约详情可见；uk(verification) 允许修订。
type FeedbackService struct {
	db     *gorm.DB
	notify *NotifyService
}

func NewFeedbackService(db *gorm.DB) *FeedbackService { return &FeedbackService{db: db} }

// WithNotify 装配订阅消息（新反馈通知学员）
func (s *FeedbackService) WithNotify(n *NotifyService) *FeedbackService {
	s.notify = n
	return s
}

// Write 教练提交/修订课后反馈。核销单必须归属该教练（他人核销 404 隔离）。
func (s *FeedbackService) Write(ctx context.Context, coachID, verificationID int64, content string) (*model.ClassFeedback, error) {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) == 0 {
		return nil, errs.BadParam("反馈内容不能为空")
	}
	if len(runes) > 1000 {
		runes = runes[:1000]
	}
	content = string(runes)
	var v model.Verification
	if err := s.db.WithContext(ctx).First(&v, verificationID).Error; err != nil {
		return nil, errs.NotFound("核销记录不存在")
	}
	if v.CoachID != coachID {
		return nil, errs.NotFound("核销记录不存在")
	}
	if v.Status != model.VerifyStatusVerified {
		return nil, errs.Biz("课程核销完成后才能填写反馈")
	}
	var fb model.ClassFeedback
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("verification_id = ?", verificationID).First(&fb).Error; err == nil {
			fb.Content = content
			return tx.Save(&fb).Error
		}
		fb = model.ClassFeedback{
			VerificationID: verificationID,
			CoachID:        coachID, UserID: v.UserID, Content: content,
		}
		return tx.Create(&fb).Error
	})
	if err != nil {
		return nil, errs.Server(err, "保存反馈失败")
	}
	tryNotify(ctx, s.notify, v.UserID, model.NotifySceneFeedbackNew, fb.ID,
		map[string]string{"thing1": "教练已填写课后反馈，点击查看"})
	return &fb, nil
}

// OfVerification 单条核销的反馈（给预约/团课详情展示）。
// 无反馈返回 (nil, nil)；DB 错误上抛（P0-3：原实现把 DB 错误也吞成 nil，调用方解引用必崩）。
func (s *FeedbackService) OfVerification(ctx context.Context, verificationID int64) (*model.ClassFeedback, error) {
	var fb model.ClassFeedback
	if err := s.db.WithContext(ctx).Where("verification_id = ?", verificationID).First(&fb).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &fb, nil
}

// OfBooking 私教预约关联反馈（经核销单关联，校验归属）
func (s *FeedbackService) OfBooking(ctx context.Context, userID, bookingID int64) map[string]interface{} {
	var v model.Verification
	if err := s.db.WithContext(ctx).
		Where("biz_type = ? AND booking_id = ? AND user_id = ?", "private", bookingID, userID).
		First(&v).Error; err != nil {
		return nil
	}
	fb, err := s.OfVerification(ctx, v.ID)
	if err != nil || fb == nil {
		return nil
	}
	return map[string]interface{}{"content": fb.Content, "createdAt": fb.CreatedAt, "verificationId": fb.ID}
}

// OfGroupClass 团课关联反馈（经我的报名核销单关联）
func (s *FeedbackService) OfGroupClass(ctx context.Context, userID, classID int64) map[string]interface{} {
	var v model.Verification
	err := s.db.WithContext(ctx).
		Where("biz_type = ? AND user_id = ?", "group", userID).
		Where("enrollment_id = (SELECT id FROM group_enrollment WHERE group_class_id = ? AND user_id = ?)", classID, userID).
		First(&v).Error
	if err != nil {
		return nil
	}
	fb, err := s.OfVerification(ctx, v.ID)
	if err != nil || fb == nil {
		return nil
	}
	return map[string]interface{}{"content": fb.Content, "createdAt": fb.CreatedAt, "verificationId": fb.ID}
}

// Mine 学员的反馈列表（含教练名与课程信息，最新在前）
func (s *FeedbackService) Mine(ctx context.Context, userID int64) ([]map[string]interface{}, error) {
	var fbs []model.ClassFeedback
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("id DESC").Limit(100).Find(&fbs).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(fbs))
	for _, fb := range fbs {
		var coach model.Coach
		s.db.WithContext(ctx).Select("id, name").First(&coach, fb.CoachID)
		item := map[string]interface{}{
			"feedback": fb, "coachName": coach.Name,
		}
		var v model.Verification
		if err := s.db.WithContext(ctx).First(&v, fb.VerificationID).Error; err == nil {
			item["bizType"] = v.BizType
			item["verifiedAt"] = v.VerifiedAt
		}
		out = append(out, item)
	}
	return out, nil
}

// CoachVerifications 教练端：近期已核销列表（附已有反馈），供「学员反馈」页
func (s *FeedbackService) CoachVerifications(ctx context.Context, coachID int64) ([]map[string]interface{}, error) {
	var vs []model.Verification
	if err := s.db.WithContext(ctx).
		Where("coach_id = ? AND status = ?", coachID, model.VerifyStatusVerified).
		Order("verified_at DESC").Limit(50).Find(&vs).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(vs))
	for _, v := range vs {
		var user model.User
		s.db.WithContext(ctx).Select("id, nickname").First(&user, v.UserID)
		fb, err := s.OfVerification(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		// 已核销未写反馈是该列表常态：输出空对象，不能解引用 nil（P0-3）
		feedback := map[string]interface{}{"content": "", "createdAt": nil}
		if fb != nil {
			feedback = map[string]interface{}{"content": fb.Content, "createdAt": fb.CreatedAt}
		}
		out = append(out, map[string]interface{}{
			"verificationId": v.ID, "bizType": v.BizType, "verifiedAt": v.VerifiedAt,
			"userName": user.Nickname,
			"feedback": feedback,
		})
	}
	return out, nil
}

// ---- 体重记录 ----

// BodyService 体重记录（Keep 风格体重曲线的数据源）
type BodyService struct{ db *gorm.DB }

func NewBodyService(db *gorm.DB) *BodyService { return &BodyService{db: db} }

// AddMetric 记录体重（同日 upsert）。weightG 克整数，100g 粒度，20~300kg。
func (s *BodyService) AddMetric(ctx context.Context, userID int64, date string, weightG int) (*model.BodyMetric, error) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, errs.BadParam("日期格式应为 YYYY-MM-DD")
	}
	if weightG < 20000 || weightG > 300000 {
		return nil, errs.BadParam("体重需在 20kg ~ 300kg 之间")
	}
	if weightG%100 != 0 {
		weightG = weightG / 100 * 100 // 0.1kg 粒度
	}
	var m model.BodyMetric
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND record_date = ?", userID, date).First(&m).Error; err == nil {
			m.WeightG = weightG
			return tx.Save(&m).Error
		}
		m = model.BodyMetric{UserID: userID, RecordDate: date, WeightG: weightG}
		return tx.Create(&m).Error
	})
	if err != nil {
		return nil, errs.Server(err, "保存体重失败")
	}
	return &m, nil
}

// ListMetrics 体重列表（升序，默认近 90 条）
func (s *BodyService) ListMetrics(ctx context.Context, userID int64, limit int) ([]model.BodyMetric, error) {
	if limit < 1 || limit > 365 {
		limit = 90
	}
	var rows []model.BodyMetric
	// 先取最近 N 条再升序输出（曲线从左到右）
	err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).Order("record_date DESC").Limit(limit).
		Find(&rows).Error
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, err
}
