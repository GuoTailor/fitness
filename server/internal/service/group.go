package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
)

// GroupService 团课：发布/报名（乐观锁防超卖）/退出（时限规则）/取消（批量返还）。
type GroupService struct {
	db     *gorm.DB
	cfg    *ConfigService
	notify *NotifyService
}

func NewGroupService(db *gorm.DB, cfg *ConfigService) *GroupService {
	return &GroupService{db: db, cfg: cfg}
}

// WithNotify 装配订阅消息（报名成功通知学员）；不装配则静默跳过。
func (s *GroupService) WithNotify(n *NotifyService) *GroupService {
	s.notify = n
	return s
}

// Publish 教练发布团课
func (s *GroupService) Publish(ctx context.Context, coachID int64, pkgID int64, title, date string, startMinute, endMinute int, location string, capacity int, remark string, ownerID int64) (*model.GroupClass, error) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, errs.BadParam("日期格式应为 YYYY-MM-DD")
	}
	if startMinute%30 != 0 || endMinute <= startMinute || endMinute > 24*60 {
		return nil, errs.BadParam("时间不合法")
	}
	if capacity < 1 || capacity > 100 {
		return nil, errs.BadParam("容量需在 1-100 之间")
	}
	if title == "" {
		return nil, errs.BadParam("标题不能为空")
	}
	var pkg model.CoursePackage
	if err := s.db.WithContext(ctx).Where("id = ? AND status = 1", pkgID).First(&pkg).Error; err != nil {
		return nil, errs.NotFound("课包不存在或已下架")
	}
	var cnt int64
	s.db.WithContext(ctx).Model(&model.CoachPackage{}).
		Where("coach_id = ? AND package_id = ?", coachID, pkgID).Count(&cnt)
	if cnt == 0 {
		return nil, errs.Biz("你未在售此课包")
	}
	dayStart, _ := time.ParseInLocation("2006-01-02", date, time.Local)
	if dayStart.Add(time.Duration(startMinute) * time.Minute).Before(time.Now().Add(30 * time.Minute)) {
		return nil, errs.Biz("团课开始时间至少在 30 分钟后")
	}
	gc := model.GroupClass{
		OwnerID: ownerID, CoachID: coachID, PackageID: pkgID, Title: title,
		ClassDate: date, StartMinute: startMinute, EndMinute: endMinute,
		Location: location, Capacity: capacity, Status: model.GCStatusEnrolling, Remark: remark,
	}
	if err := s.db.WithContext(ctx).Create(&gc).Error; err != nil {
		return nil, errs.Server(err, "发布团课失败")
	}
	return &gc, nil
}

// ListBetween 团课日历（from/to 含端点，YYYY-MM-DD）；userID>0 时标注 myStatus
func (s *GroupService) ListBetween(ctx context.Context, from, to string, userID int64) ([]map[string]interface{}, error) {
	var classes []model.GroupClass
	err := s.db.WithContext(ctx).
		Where("class_date BETWEEN ? AND ? AND status IN ?", from, to,
			[]string{model.GCStatusEnrolling, model.GCStatusFulfilled, model.GCStatusClosed, model.GCStatusFinished}).
		Order("class_date, start_minute").Find(&classes).Error
	if err != nil {
		return nil, err
	}
	cids := make([]int64, 0, len(classes))
	for _, g := range classes {
		cids = append(cids, g.ID)
	}
	coaches := map[int64]model.Coach{}
	pkgs := map[int64]model.CoursePackage{}
	myStatus := map[int64]string{}
	if len(cids) > 0 {
		var cs []model.Coach
		s.db.WithContext(ctx).Where("id IN ?", coachIDsOf(classes)).Find(&cs)
		for _, c := range cs {
			coaches[c.ID] = c
		}
		var ps []model.CoursePackage
		s.db.WithContext(ctx).Where("id IN ?", pkgIDsOf(classes)).Find(&ps)
		for _, p := range ps {
			pkgs[p.ID] = p
		}
		if userID > 0 {
			var ges []model.GroupEnrollment
			s.db.WithContext(ctx).Where("user_id = ? AND group_class_id IN ?", userID, cids).Find(&ges)
			for _, ge := range ges {
				myStatus[ge.GroupClassID] = ge.Status
			}
		}
	}
	out := make([]map[string]interface{}, 0, len(classes))
	for _, g := range classes {
		out = append(out, map[string]interface{}{
			"class": g, "coachName": coaches[g.CoachID].Name, "coachAvatar": coaches[g.CoachID].AvatarURL,
			"packageTitle": pkgs[g.PackageID].Title, "myStatus": myStatus[g.ID],
		})
	}
	return out, nil
}

func coachIDsOf(gs []model.GroupClass) []int64 {
	m := map[int64]bool{}
	for _, g := range gs {
		m[g.CoachID] = true
	}
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
func pkgIDsOf(gs []model.GroupClass) []int64 {
	m := map[int64]bool{}
	for _, g := range gs {
		m[g.PackageID] = true
	}
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Detail 团课详情（含我的报名状态，userID 可为 0）
func (s *GroupService) Detail(ctx context.Context, classID, userID int64) (map[string]interface{}, error) {
	var gc model.GroupClass
	if err := s.db.WithContext(ctx).First(&gc, classID).Error; err != nil {
		return nil, errs.NotFound("团课不存在")
	}
	var coach model.Coach
	s.db.WithContext(ctx).First(&coach, gc.CoachID)
	var pkg model.CoursePackage
	s.db.WithContext(ctx).First(&pkg, gc.PackageID)
	myStatus := ""
	if userID > 0 {
		var ge model.GroupEnrollment
		if err := s.db.WithContext(ctx).
			Where("group_class_id = ? AND user_id = ?", classID, userID).First(&ge).Error; err == nil {
			myStatus = ge.Status
		}
	}
	return map[string]interface{}{
		"class": gc, "coach": coach, "package": pkg, "myStatus": myStatus,
	}, nil
}

// Enroll 报名：乐观锁占位 + 扣次，退出后重报翻回复用行
func (s *GroupService) Enroll(ctx context.Context, classID, userID, memberPackageID int64) (*model.GroupEnrollment, error) {
	var gc model.GroupClass
	if err := s.db.WithContext(ctx).First(&gc, classID).Error; err != nil {
		return nil, errs.NotFound("团课不存在")
	}
	if gc.Status != model.GCStatusEnrolling {
		return nil, errs.Biz("该团课当前不可报名")
	}
	startAt, _ := time.ParseInLocation("2006-01-02 15:04", fmt.Sprintf("%s %02d:%02d", dateOnly(gc.ClassDate), gc.StartMinute/60, gc.StartMinute%60), time.Local)
	if startAt.Before(time.Now()) {
		return nil, errs.Biz("团课已开始")
	}
	var mp model.MemberPackage
	if err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", memberPackageID, userID).First(&mp).Error; err != nil {
		return nil, errs.NotFound("课时账户不存在")
	}
	if mp.Status != model.MPStatusInactive && mp.Status != model.MPStatusActive {
		return nil, errs.Biz("该课包当前状态不可用")
	}
	if mp.ExpireDate != nil && mp.ExpireDate.Format("2006-01-02") < time.Now().Format("2006-01-02") {
		return nil, errs.Biz("该课包已过期") // M3：状态未及时清扫时按有效期硬拒
	}
	if mp.PackageID != gc.PackageID {
		return nil, errs.Biz("请使用该团课对应课包的课时报名")
	}
	if mp.RemainingSessions() < 1 {
		return nil, errs.Biz("剩余课时不足")
	}

	var enrollment model.GroupEnrollment
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 复用行（quit → enrolled）或新建；同课同人唯一。
		// 复用时 enroll_seq+1（P0-4）：扣次/返还幂等键带世代，重报不再撞键
		if err := tx.Where("group_class_id = ? AND user_id = ?", classID, userID).First(&enrollment).Error; err == nil {
				if enrollment.Status == model.GEStatusEnrolled || enrollment.Status == model.GEStatusVerified {
					return errs.Conflict("你已报名该团课")
				}
				nextSeq := enrollment.EnrollSeq + 1
				res := tx.Model(&model.GroupEnrollment{}).
					Where("id = ? AND status = ? AND enroll_seq = ?", enrollment.ID, model.GEStatusQuit, enrollment.EnrollSeq).
					Updates(map[string]interface{}{
						"status": model.GEStatusEnrolled, "quit_time": nil,
						"member_package_id": memberPackageID, "enroll_seq": nextSeq,
					})
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected == 0 {
					return errs.Conflict("报名状态变化，请刷新")
				}
				enrollment.EnrollSeq = nextSeq
			} else {
				enrollment = model.GroupEnrollment{
					GroupClassID: classID, UserID: userID, MemberPackageID: memberPackageID,
					Status: model.GEStatusEnrolled, EnrollSeq: 1,
				}
				if err := tx.Create(&enrollment).Error; err != nil {
					if isDuplicateKey(err) {
						return errs.Conflict("你已报名该团课")
					}
					return err
				}
			}
			// 乐观锁防超卖：占位成功才算报上
			res := tx.Model(&model.GroupClass{}).
				Where("id = ? AND status = ? AND enrolled_count < capacity", classID, model.GCStatusEnrolling).
				Updates(map[string]interface{}{
					"enrolled_count": gorm.Expr("enrolled_count + 1"),
					"status":         gorm.Expr("CASE WHEN enrolled_count + 1 >= capacity THEN 'fulfilled' ELSE 'enrolling' END"),
				})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return errs.Conflict("名额已满")
			}
			// 扣次（幂等键 = 报名 ID + 世代）
			return deductOne(tx, &mp, model.TxnTypeDeduct,
				fmt.Sprintf("group_deduct:%d:%d", enrollment.ID, enrollment.EnrollSeq),
				"group_enrollment", enrollment.ID, "团课报名扣次")
		})
	if err != nil {
		return nil, err
	}
	tryNotify(ctx, s.notify, userID, model.NotifySceneBookingConfirmed, enrollment.ID,
		map[string]string{"thing1": "团课报名成功", "date2": dateOnly(gc.ClassDate)})
	return &enrollment, nil
}

// Quit 退出：开课前 ≥X 小时可退并返还课时；临近开课不可退
func (s *GroupService) Quit(ctx context.Context, classID, userID int64) error {
	var gc model.GroupClass
	if err := s.db.WithContext(ctx).First(&gc, classID).Error; err != nil {
		return errs.NotFound("团课不存在")
	}
	var ge model.GroupEnrollment
	if err := s.db.WithContext(ctx).
		Where("group_class_id = ? AND user_id = ?", classID, userID).First(&ge).Error; err != nil {
		return errs.NotFound("未报名该团课")
	}
	if ge.Status != model.GEStatusEnrolled {
		return errs.Biz("当前状态不可退出")
	}
	hours := s.cfg.GetInt(ctx, model.CfgGroupQuitDeadlineHours, 4)
	startAt, _ := time.ParseInLocation("2006-01-02 15:04", fmt.Sprintf("%s %02d:%02d", dateOnly(gc.ClassDate), gc.StartMinute/60, gc.StartMinute%60), time.Local)
	if startAt.Sub(time.Now()) < time.Duration(hours)*time.Hour {
		return errs.Biz(fmt.Sprintf("开课前 %d 小时内不可退出", hours))
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.GroupEnrollment{}).
			Where("id = ? AND status = ?", ge.ID, model.GEStatusEnrolled).
			Updates(map[string]interface{}{"status": model.GEStatusQuit, "quit_time": time.Now()})
		if res.RowsAffected == 0 {
			return errs.Conflict("状态变化，请刷新")
		}
		// 释放名额；仅报名中/满员状态回到「报名中」（E5：closed/finished/cancelled 不复活）
		tx.Model(&model.GroupClass{}).
			Where("id = ? AND enrolled_count > 0 AND status IN ?", classID,
				[]string{model.GCStatusEnrolling, model.GCStatusFulfilled}).
			Updates(map[string]interface{}{
				"enrolled_count": gorm.Expr("enrolled_count - 1"),
				"status":         model.GCStatusEnrolling,
			})
		// 作废未核销的核销码（对照私教取消的完整撤销）
		tx.Model(&model.Verification{}).
			Where("biz_type = ? AND enrollment_id = ? AND status = ?", "group", ge.ID, model.VerifyStatusPending).
			Update("status", model.VerifyStatusExpired)
		var mp model.MemberPackage
		if err := tx.First(&mp, ge.MemberPackageID).Error; err == nil && mp.Status != model.MPStatusRefunded {
			return returnOne(tx, &mp, fmt.Sprintf("group_refund:%d:%d", ge.ID, ge.EnrollSeq),
				"group_enrollment", ge.ID, "退出团课返还")
		}
		return nil
	})
}

// CancelByCoach 教练取消团课：全部报名学员返还课时
func (s *GroupService) CancelByCoach(ctx context.Context, classID, coachID int64, reason string) error {
	var gc model.GroupClass
	if err := s.db.WithContext(ctx).First(&gc, classID).Error; err != nil {
		return errs.NotFound("团课不存在")
	}
	if coachID > 0 && gc.CoachID != coachID {
		return errs.Forbidden("只能取消自己的团课")
	}
	res := s.db.WithContext(ctx).Model(&model.GroupClass{}).
		Where("id = ? AND status IN ?", classID,
			[]string{model.GCStatusEnrolling, model.GCStatusFulfilled, model.GCStatusClosed}).
		Updates(map[string]interface{}{"status": model.GCStatusCancelled, "cancel_reason": reason})
	if res.RowsAffected == 0 {
		return errs.Conflict("该团课当前不可取消")
	}
	var enrollments []model.GroupEnrollment
	s.db.WithContext(ctx).Where("group_class_id = ? AND status = ?", classID, model.GEStatusEnrolled).Find(&enrollments)
	// E6：逐人返还失败不再静默吞掉——收集失败并提示人工介入
	failed := 0
	for _, ge := range enrollments {
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			tx.Model(&model.GroupEnrollment{}).Where("id = ? AND status = ?", ge.ID, model.GEStatusEnrolled).
				Update("status", model.GEStatusQuit)
			// 作废未核销的核销码
			tx.Model(&model.Verification{}).
				Where("biz_type = ? AND enrollment_id = ? AND status = ?", "group", ge.ID, model.VerifyStatusPending).
				Update("status", model.VerifyStatusExpired)
			var mp model.MemberPackage
			if err := tx.First(&mp, ge.MemberPackageID).Error; err == nil && mp.Status != model.MPStatusRefunded {
				return returnOne(tx, &mp, fmt.Sprintf("group_cancel:%d:%d", classID, ge.UserID), "group_class", classID, "团课取消返还")
			}
			return nil
		})
		if err != nil {
			failed++
			log.Printf("[group] 团课 %d 取消时学员 %d 课时返还失败: %v", classID, ge.UserID, err)
		}
	}
	if failed > 0 {
		return errs.Biz(fmt.Sprintf("%d 名学员的课时返还失败，请重试取消或人工核对流水", failed))
	}
	return nil
}

// EnrollmentsOf 报名名单（教练视角）
func (s *GroupService) EnrollmentsOf(ctx context.Context, classID int64) ([]map[string]interface{}, error) {
	var ges []model.GroupEnrollment
	s.db.WithContext(ctx).Where("group_class_id = ? AND status IN ?", classID,
		[]string{model.GEStatusEnrolled, model.GEStatusVerified}).Order("id").Find(&ges)
	uids := make([]int64, 0, len(ges))
	for _, g := range ges {
		uids = append(uids, g.UserID)
	}
	users := map[int64]model.User{}
	if len(uids) > 0 {
		var us []model.User
		s.db.WithContext(ctx).Where("id IN ?", uids).Find(&us)
		for _, u := range us {
			users[u.ID] = u
		}
	}
	out := make([]map[string]interface{}, 0, len(ges))
	for _, ge := range ges {
		u := users[ge.UserID]
		out = append(out, map[string]interface{}{
			"enrollment": ge, "userName": u.Nickname, "userPhone": u.Phone,
		})
	}
	return out, nil
}

// GenCode 团课报名核销码
func (s *GroupService) GenCode(ctx context.Context, classID, userID int64) (*model.Verification, error) {
	var ge model.GroupEnrollment
	if err := s.db.WithContext(ctx).
		Where("group_class_id = ? AND user_id = ?", classID, userID).First(&ge).Error; err != nil {
		return nil, errs.NotFound("未报名该团课")
	}
	if ge.Status != model.GEStatusEnrolled {
		return nil, errs.Biz("当前状态不可核销")
	}
	var gc model.GroupClass
	s.db.WithContext(ctx).First(&gc, classID)
	var exist model.Verification
	err := s.db.WithContext(ctx).
		Where("biz_type = ? AND enrollment_id = ? AND status = ?", "group", ge.ID, model.VerifyStatusPending).
		First(&exist).Error
	if err == nil {
		return &exist, nil
	}
	v := model.Verification{
		BizType: "group", EnrollmentID: ge.ID, UserID: userID, CoachID: gc.CoachID,
		MemberPackageID: ge.MemberPackageID, Sessions: 1,
		Code: randomCode("G"), Status: model.VerifyStatusPending,
	}
	if err := s.db.WithContext(ctx).Create(&v).Error; err != nil {
		return nil, errs.Server(err, "生成核销码失败")
	}
	return &v, nil
}

// CoachClasses 教练自己的团课列表
func (s *GroupService) CoachClasses(ctx context.Context, coachID int64, from, to string) ([]model.GroupClass, error) {
	var list []model.GroupClass
	q := s.db.WithContext(ctx).Where("coach_id = ?", coachID)
	if from != "" && to != "" {
		q = q.Where("class_date BETWEEN ? AND ?", from, to)
	}
	err := q.Order("class_date DESC, start_minute DESC").Limit(100).Find(&list).Error
	return list, err
}
