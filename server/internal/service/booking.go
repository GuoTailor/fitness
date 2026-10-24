package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
)

// BookingService 私教预约。
// 课时模型：提交预约即扣次（预约占用 1 课时），开课前 ≥X 小时取消返还，
// X 小时内取消扣次不返还（penalty）；教练/后台取消一律返还。
// 防冲突：事务内区间重叠检查 + slot_token 唯一键兜底（取消/完成时置换释放）。
type BookingService struct {
	db     *gorm.DB
	cfg    *ConfigService
	notify *NotifyService
}

func NewBookingService(db *gorm.DB, cfg *ConfigService) *BookingService {
	return &BookingService{db: db, cfg: cfg}
}

// WithNotify 装配订阅消息（预约通知教练、确认通知学员）；不装配则静默跳过。
func (s *BookingService) WithNotify(n *NotifyService) *BookingService {
	s.notify = n
	return s
}

const lessonMinutes = 60 // P0 私教课固定 60 分钟（两个 30 分钟时段格）

// Slot 某日某教练的可约起始格
type Slot struct {
	StartMinute int    `json:"startMinute"`
	EndMinute   int    `json:"endMinute"`
	Label       string `json:"label"`
}

// SlotsOfDate 计算教练某日可约时段：每周模板 → 60 分钟课需完整落在单个窗口内，
// 排除与既有有效预约重叠的格、过去时间及 1 小时内的临场格。
func (s *BookingService) SlotsOfDate(ctx context.Context, coachID int64, date string) ([]Slot, error) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, errs.BadParam("日期格式应为 YYYY-MM-DD")
	}
	var schedules []model.CoachSchedule
	s.db.WithContext(ctx).Where("coach_id = ? AND status = 1", coachID).Order("start_minute").Find(&schedules)
	if len(schedules) == 0 {
		return nil, nil
	}
	// 当日有效预约（占用的区间）。completed 的 slot_token 已置换为 done:{id}，
	// 唯一键层面可再约（核销释放时段给下一位学员），列表须与之一致；
	var bookings []model.Booking
	s.db.WithContext(ctx).
		Where("coach_id = ? AND booking_date = ? AND status IN ?", coachID, date,
			[]string{model.BookingStatusPending, model.BookingStatusConfirmed}).
		Find(&bookings)

	dayStart, _ := time.ParseInLocation("2006-01-02", date, time.Local)
	now := time.Now()
	slots := []Slot{}
	for _, sc := range schedules {
		for st := sc.StartMinute; st+lessonMinutes <= sc.EndMinute; st += 30 {
			startAt := dayStart.Add(time.Duration(st) * time.Minute)
			if startAt.Before(now.Add(time.Hour)) { // 需至少提前 1 小时
				continue
			}
			end := st + lessonMinutes
			overlap := false
			for _, b := range bookings {
				if b.StartMinute < end && b.EndMinute > st {
					overlap = true
					break
				}
			}
			if overlap {
				continue
			}
			slots = append(slots, Slot{StartMinute: st, EndMinute: end, Label: minuteLabel(st)})
		}
	}
	return slots, nil
}

func minuteLabel(m int) string {
	return fmt.Sprintf("%02d:%02d", m/60, m%60)
}

// dateOnly 规范化从数据库读回的 DATE 字符串：
// MySQL 返回 "2006-01-02"，sqlite 可能带 " 00:00:00+08:00" 后缀，统一截前 10 位。
func dateOnly(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// Create 提交预约（扣次）。admin 代客时 operator 为 admin。
func (s *BookingService) Create(ctx context.Context, userID, coachID, memberPackageID int64, date string, startMinute int) (*model.Booking, error) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, errs.BadParam("日期格式应为 YYYY-MM-DD")
	}
	if startMinute%30 != 0 || startMinute < 0 || startMinute+lessonMinutes > 24*60 {
		return nil, errs.BadParam("时间格不合法")
	}
	var coach model.Coach
	if err := s.db.WithContext(ctx).Where("id = ? AND status = 1", coachID).First(&coach).Error; err != nil {
		return nil, errs.NotFound("教练不存在或已停用")
	}
	var mp model.MemberPackage
	if err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", memberPackageID, userID).First(&mp).Error; err != nil {
		return nil, errs.NotFound("课时账户不存在")
	}
	if mp.Status == model.MPStatusRefunded {
		return nil, errs.Biz("该课包已退款")
	}
	if mp.Status == model.MPStatusExpired {
		return nil, errs.Biz("该课包已过期")
	}
	if mp.RemainingSessions() < 1 {
		return nil, errs.Biz("剩余课时不足")
	}
	var pkg model.CoursePackage
	if err := s.db.WithContext(ctx).Where("id = ?", mp.PackageID).First(&pkg).Error; err != nil {
		return nil, errs.NotFound("课包不存在")
	}
	// 教练须在售该课包
	var cnt int64
	s.db.WithContext(ctx).Model(&model.CoachPackage{}).
		Where("coach_id = ? AND package_id = ?", coachID, mp.PackageID).Count(&cnt)
	if cnt == 0 {
		return nil, errs.Biz("该教练未在售此课包，请选择其他教练")
	}
	// 时段需在教练模板窗口内
	dayStart, _ := time.ParseInLocation("2006-01-02", date, time.Local)
	if dayStart.Add(time.Duration(startMinute) * time.Minute).Before(time.Now().Add(time.Hour)) {
		return nil, errs.Biz("需至少提前 1 小时预约")
	}
	weekday := int(dayStart.Weekday())
	var windows []model.CoachSchedule
	s.db.WithContext(ctx).Where("coach_id = ? AND weekday = ? AND status = 1", coachID, weekday).Find(&windows)
	inWindow := false
	for _, w := range windows {
		if startMinute >= w.StartMinute && startMinute+lessonMinutes <= w.EndMinute {
			inWindow = true
			break
		}
	}
	if !inWindow {
		return nil, errs.Biz("该时间不在教练可约时段内")
	}

	var booking model.Booking
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 区间重叠快速失败（唯一键是最终兜底）。completed 已释放 slot_token，
		// 同一时段可接新学员，不视为占用
		var overlapCnt int64
		tx.Model(&model.Booking{}).
			Where("coach_id = ? AND booking_date = ? AND status IN ? AND start_minute < ? AND end_minute > ?",
				coachID, date,
				[]string{model.BookingStatusPending, model.BookingStatusConfirmed},
				startMinute+lessonMinutes, startMinute).
			Count(&overlapCnt)
		if overlapCnt > 0 {
			return errs.Conflict("该时段刚被占用，请刷新选择其他时段")
		}
		booking = model.Booking{
			OwnerID: coach.OwnerID, UserID: userID, CoachID: coachID,
			PackageID: mp.PackageID, MemberPackageID: mp.ID,
			BookingDate: date, StartMinute: startMinute, EndMinute: startMinute + lessonMinutes,
			SlotToken: model.SlotTokenFor(coachID, date, startMinute),
			Status:    model.BookingStatusPending,
		}
		if err := tx.Create(&booking).Error; err != nil {
			if isDuplicateKey(err) {
				return errs.Conflict("该时段刚被占用，请刷新选择其他时段")
			}
			return err
		}
		// 扣次（含首次扣次开卡），条件更新防并发超扣
		if err := deductOne(tx, &mp, model.TxnTypeDeduct, "booking_deduct:"+fmt.Sprint(booking.ID),
			"booking", booking.ID, "预约扣次"); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 新预约 → 通知教练（其绑定的登录账号；未绑定则跳过）
	var coachUserID int64
	s.db.WithContext(ctx).Model(&model.Coach{}).
		Select("user_id").Where("id = ?", coachID).Scan(&coachUserID)
	if coachUserID > 0 {
		tryNotify(ctx, s.notify, coachUserID, model.NotifySceneBookingNew, booking.ID,
			map[string]string{"thing1": "您有新的私教预约", "date2": date})
	}
	return &booking, nil
}

// deductOne 扣 1 课时：条件更新余额 + 写流水（唯一键幂等）；inactive 首扣时开卡。
func deductOne(tx *gorm.DB, mp *model.MemberPackage, txnType, idemKey, bizType string, bizID int64, remark string) error {
	res := tx.Model(&model.MemberPackage{}).
		Where("id = ? AND used_sessions < total_sessions AND status IN ?", mp.ID,
			[]string{model.MPStatusInactive, model.MPStatusActive, model.MPStatusExhausted}).
		Updates(map[string]interface{}{"used_sessions": gorm.Expr("used_sessions + 1")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errs.Biz("剩余课时不足")
	}
	// 首次扣次 → 开卡
	tx.Model(&model.MemberPackage{}).
		Where("id = ? AND status = ?", mp.ID, model.MPStatusInactive).
		Updates(map[string]interface{}{
			"status": model.MPStatusActive,
			"activated_at": time.Now(),
			"expire_date":  time.Now().AddDate(0, 0, mp.ValidDays),
		})
	if err := tx.Create(&model.PackageTransaction{
		MemberPackageID: mp.ID, UserID: mp.UserID, SessionsChange: -1,
		Type: txnType, IdempotencyKey: idemKey, BizType: bizType, BizID: bizID, Remark: remark,
	}).Error; err != nil && !isDuplicateKey(err) {
		return err
	}
	// 用完 → exhausted
	tx.Model(&model.MemberPackage{}).
		Where("id = ? AND used_sessions >= total_sessions AND status = ?", mp.ID, model.MPStatusActive).
		Update("status", model.MPStatusExhausted)
	return nil
}

// returnOne 返还 1 课时（幂等）
func returnOne(tx *gorm.DB, mp *model.MemberPackage, idemKey, bizType string, bizID int64, remark string) error {
	if err := tx.Create(&model.PackageTransaction{
		MemberPackageID: mp.ID, UserID: mp.UserID, SessionsChange: 1,
		Type: model.TxnTypeBookingRefund, IdempotencyKey: idemKey, BizType: bizType, BizID: bizID, Remark: remark,
	}).Error; err != nil {
		if isDuplicateKey(err) {
			return nil // 已返还过
		}
		return err
	}
	res := tx.Model(&model.MemberPackage{}).
		Where("id = ?", mp.ID).
		Updates(map[string]interface{}{
			"used_sessions": gorm.Expr("CASE WHEN used_sessions > 0 THEN used_sessions - 1 ELSE 0 END"),
			"status":        model.MPStatusActive,
		})
	return res.Error
}

// MyBookings 学员预约列表
func (s *BookingService) MyBookings(ctx context.Context, userID int64, status string) ([]model.Booking, error) {
	q := s.db.WithContext(ctx).Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var list []model.Booking
	err := q.Order("booking_date DESC, start_minute DESC").Limit(100).Find(&list).Error
	return list, err
}

// UserCancel 学员取消：开课前 ≥X 小时返还；X 小时内不返还（penalty）
func (s *BookingService) UserCancel(ctx context.Context, bookingID, userID int64) error {
	return s.cancel(ctx, bookingID, userID, "user")
}

// CoachCancel 教练取消/拒绝：一律返还学员课时
func (s *BookingService) CoachCancel(ctx context.Context, bookingID, coachID int64, reject bool) error {
	var b model.Booking
	if err := s.db.WithContext(ctx).First(&b, bookingID).Error; err != nil {
		return errs.NotFound("预约不存在")
	}
	if b.CoachID != coachID {
		return errs.Forbidden("只能操作自己的预约")
	}
	return s.cancel(ctx, bookingID, b.UserID, "coach")
}

// AdminCancel 后台代取消
func (s *BookingService) AdminCancel(ctx context.Context, bookingID int64) error {
	var b model.Booking
	if err := s.db.WithContext(ctx).First(&b, bookingID).Error; err != nil {
		return errs.NotFound("预约不存在")
	}
	return s.cancel(ctx, bookingID, b.UserID, "admin")
}

func (s *BookingService) cancel(ctx context.Context, bookingID, userID int64, by string) error {
	var b model.Booking
	if err := s.db.WithContext(ctx).Where("id = ?", bookingID).First(&b).Error; err != nil {
		return errs.NotFound("预约不存在")
	}
	if by == "user" && b.UserID != userID {
		return errs.NotFound("预约不存在") // 隔离：不暴露他人资源
	}
	if b.Status != model.BookingStatusPending && b.Status != model.BookingStatusConfirmed {
		return errs.Biz("当前状态不可取消")
	}
	// 状态闸门：抢占取消权
	res := s.db.WithContext(ctx).Model(&model.Booking{}).
		Where("id = ? AND status IN ?", bookingID,
			[]string{model.BookingStatusPending, model.BookingStatusConfirmed}).
		Updates(map[string]interface{}{
			"status": model.BookingStatusCancelled, "cancel_by": by, "cancel_time": time.Now(),
		})
	if res.RowsAffected == 0 {
		return errs.Conflict("该预约已被处理")
	}
	// 释放时段
	s.db.WithContext(ctx).Model(&model.Booking{}).Where("id = ?", bookingID).
		Update("slot_token", model.ReleasedSlotToken(bookingID))
	// 作废未用核销码
	s.db.WithContext(ctx).Model(&model.Verification{}).
		Where("booking_id = ? AND status = ?", bookingID, model.VerifyStatusPending).
		Update("status", model.VerifyStatusCancelled)

	if by == "user" {
		hours := s.cfg.GetInt(ctx, model.CfgBookingCancelPenaltyHours, 4)
		if s.classStart(&b).Sub(time.Now()) < time.Duration(hours)*time.Hour {
			// X 小时内：课时不返还（已扣即罚）
			s.db.WithContext(ctx).Model(&model.Booking{}).Where("id = ?", bookingID).
				Update("penalty_sessions", 1)
			return nil
		}
	}
	var mp model.MemberPackage
	if err := s.db.WithContext(ctx).First(&mp, b.MemberPackageID).Error; err == nil && mp.Status != model.MPStatusRefunded {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return returnOne(tx, &mp, "booking_refund:"+fmt.Sprint(bookingID), "booking", bookingID, "取消预约返还")
		})
	}
	return nil
}

func (s *BookingService) classStart(b *model.Booking) time.Time {
	dayStart, _ := time.ParseInLocation("2006-01-02", dateOnly(b.BookingDate), time.Local)
	return dayStart.Add(time.Duration(b.StartMinute) * time.Minute)
}

// Confirm 教练确认预约
func (s *BookingService) Confirm(ctx context.Context, bookingID, coachID int64) error {
	var b model.Booking
	if err := s.db.WithContext(ctx).First(&b, bookingID).Error; err != nil {
		return errs.NotFound("预约不存在")
	}
	if b.CoachID != coachID {
		return errs.Forbidden("只能操作自己的预约")
	}
	res := s.db.WithContext(ctx).Model(&model.Booking{}).
		Where("id = ? AND status = ?", bookingID, model.BookingStatusPending).
		Update("status", model.BookingStatusConfirmed)
	if res.RowsAffected == 0 {
		return errs.Conflict("该预约不在待确认状态")
	}
	tryNotify(ctx, s.notify, b.UserID, model.NotifySceneBookingConfirmed, b.ID,
		map[string]string{"thing1": "教练已确认您的预约", "date2": dateOnly(b.BookingDate)})
	return nil
}

// CoachDay 教练当日课表（含学员信息）
func (s *BookingService) CoachDay(ctx context.Context, coachID int64, date string) ([]map[string]interface{}, error) {
	var bookings []model.Booking
	s.db.WithContext(ctx).
		Where("coach_id = ? AND booking_date = ? AND status IN ?", coachID, date,
			[]string{model.BookingStatusPending, model.BookingStatusConfirmed, model.BookingStatusCompleted}).
		Order("start_minute").Find(&bookings)
	uids := map[int64]bool{}
	for _, b := range bookings {
		uids[b.UserID] = true
	}
	users := map[int64]model.User{}
	if len(uids) > 0 {
		var us []model.User
		ids := make([]int64, 0, len(uids))
		for id := range uids {
			ids = append(ids, id)
		}
		s.db.WithContext(ctx).Where("id IN ?", ids).Find(&us)
		for _, u := range us {
			users[u.ID] = u
		}
	}
	out := make([]map[string]interface{}, 0, len(bookings))
	for _, b := range bookings {
		u := users[b.UserID]
		out = append(out, map[string]interface{}{
			"booking": b, "userName": u.Nickname, "userPhone": u.Phone, "startLabel": minuteLabel(b.StartMinute),
		})
	}
	return out, nil
}

// GenCode 生成（或复用）预约的核销码
func (s *BookingService) GenCode(ctx context.Context, bookingID, userID int64) (*model.Verification, error) {
	var b model.Booking
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", bookingID, userID).First(&b).Error; err != nil {
		return nil, errs.NotFound("预约不存在")
	}
	if b.Status != model.BookingStatusPending && b.Status != model.BookingStatusConfirmed {
		return nil, errs.Biz("当前状态不可核销")
	}
	var exist model.Verification
	err := s.db.WithContext(ctx).
		Where("biz_type = ? AND booking_id = ? AND status = ?", "private", bookingID, model.VerifyStatusPending).
		First(&exist).Error
	if err == nil {
		return &exist, nil
	}
	v := model.Verification{
		BizType: "private", BookingID: bookingID, UserID: userID, CoachID: b.CoachID,
		MemberPackageID: b.MemberPackageID, Sessions: 1,
		Code: randomCode("V"), Status: model.VerifyStatusPending,
	}
	if err := s.db.WithContext(ctx).Create(&v).Error; err != nil {
		return nil, errs.Server(err, "生成核销码失败")
	}
	return &v, nil
}

func randomCode(prefix string) string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// MySQL 1062 / sqlite UNIQUE / pg 23505
	return strings.Contains(msg, "Error 1062") ||
		strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "duplicate key")
}
