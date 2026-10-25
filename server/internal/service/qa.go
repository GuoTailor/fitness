package service

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
)

// QaService 课程问答区（规划 §3.6）：购前疑虑公开问答。
// 先发后审（默认 visible，后台可隐藏）；教练回复带认证标识且排序优先；
// 新提问通知课包教练（qa_new）、新回复通知提问者（qa_reply）。
type QaService struct {
	db     *gorm.DB
	notify *NotifyService
}

func NewQaService(db *gorm.DB) *QaService { return &QaService{db: db} }

func (s *QaService) WithNotify(n *NotifyService) *QaService {
	s.notify = n
	return s
}

// Ask 提问：登录即可（不限已购），可选匿名；课包须在售
func (s *QaService) Ask(ctx context.Context, userID, packageID int64, content string, anonymous bool) (*model.QaQuestion, error) {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) < 5 {
		return nil, errs.BadParam("提问内容至少 5 个字")
	}
	if len(runes) > 500 {
		return nil, errs.BadParam("提问内容最多 500 字")
	}
	var pkg model.CoursePackage
	if err := s.db.WithContext(ctx).Where("id = ? AND status = 1", packageID).First(&pkg).Error; err != nil {
		return nil, errs.NotFound("课包不存在或已下架")
	}
	q := model.QaQuestion{
		OwnerID: pkg.OwnerID, PackageID: packageID, UserID: userID,
		Content: string(runes), Status: model.QaStatusVisible,
	}
	if anonymous {
		q.Anonymous = 1
	}
	if err := s.db.WithContext(ctx).Create(&q).Error; err != nil {
		return nil, errs.Server(err, "发布提问失败")
	}
	// 新提问 → 通知课包全部教练（绑定了登录账号的）
	var coachUsers []struct{ UserID int64 }
	s.db.WithContext(ctx).
		Joins("JOIN coach_package cp ON cp.coach_id = coach.id").
		Where("cp.package_id = ? AND coach.user_id > 0 AND coach.status = 1", packageID).
		Select("coach.user_id AS user_id").Scan(&coachUsers)
	for _, cu := range coachUsers {
		tryNotify(ctx, s.notify, cu.UserID, model.NotifySceneQANew, q.ID,
			map[string]string{"thing1": "课包有新提问，来答疑吧", "date2": time.Now().Format("2006-01-02")})
	}
	return &q, nil
}

// Answer 回复：该课包关联教练（认证标识、优先展示）或已购学员；
// 讨论式追加（不唯一），回复后通知提问者。
func (s *QaService) Answer(ctx context.Context, userID, questionID int64, content string) (*model.QaAnswer, error) {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) == 0 {
		return nil, errs.BadParam("回复内容不能为空")
	}
	if len(runes) > 1000 {
		runes = runes[:1000]
	}
	var q model.QaQuestion
	if err := s.db.WithContext(ctx).First(&q, questionID).Error; err != nil {
		return nil, errs.NotFound("问题不存在")
	}
	if q.Status != model.QaStatusVisible {
		return nil, errs.Biz("该问题已隐藏，不可回复")
	}
	a := model.QaAnswer{QuestionID: questionID, UserID: userID, Content: string(runes), Status: model.QaStatusVisible}
	// 教练身份：当前用户绑定的教练在售该课包
	var coach model.Coach
	isCoach := s.db.WithContext(ctx).
		Joins("JOIN coach_package cp ON cp.coach_id = coach.id").
		Where("coach.user_id = ? AND cp.package_id = ?", userID, q.PackageID).
		First(&coach).Error == nil
	if isCoach {
		a.CoachID, a.IsCoach = coach.ID, 1
	} else {
		// 已购学员：存在已支付/退款中的该课包订单
		var cnt int64
		s.db.WithContext(ctx).Model(&model.Order{}).
			Where("user_id = ? AND package_id = ? AND status IN ?", userID, q.PackageID,
				[]string{model.OrderStatusPaid, model.OrderStatusRefund, model.OrderStatusRefunded}).
			Count(&cnt)
		if cnt == 0 {
			return nil, errs.Biz("该课包的教练或已购学员才能回复")
		}
	}
	if err := s.db.WithContext(ctx).Create(&a).Error; err != nil {
		return nil, errs.Server(err, "回复失败")
	}
	if q.UserID != userID {
		tryNotify(ctx, s.notify, q.UserID, model.NotifySceneQAReply, a.ID,
			map[string]string{"thing1": "你的提问有新回复", "date2": time.Now().Format("2006-01-02")})
	}
	return &a, nil
}

// enrichQuestions 问答列表组装：置顶优先、时间倒序；回复按 置顶>教练>时间 升序
func (s *QaService) enrichQuestions(ctx context.Context, qs []model.QaQuestion, withHiddenAnswers bool, realName bool) []map[string]interface{} {
	qids := make([]int64, 0, len(qs))
	uids := map[int64]bool{}
	for _, q := range qs {
		qids = append(qids, q.ID)
		uids[q.UserID] = true
	}
	var answers []model.QaAnswer
	if len(qids) > 0 {
		q := s.db.WithContext(ctx).Where("question_id IN ?", qids)
		if !withHiddenAnswers {
			q = q.Where("status = ?", model.QaStatusVisible)
		}
		q.Order("pinned DESC, is_coach DESC, id ASC").Limit(500).Find(&answers)
	}
	answersOf := map[int64][]map[string]interface{}{}
	for _, a := range answers {
		uids[a.UserID] = true
		answersOf[a.QuestionID] = append(answersOf[a.QuestionID], map[string]interface{}{
			"id": a.ID, "content": a.Content, "isCoach": a.IsCoach, "pinned": a.Pinned,
			"status": a.Status, "createdAt": a.CreatedAt, "userId": a.UserID, "coachId": a.CoachID,
		})
	}
	// 作者名（教练取教练名，学员取昵称）
	coachNames := map[int64]string{}
	var coaches []model.Coach
	s.db.WithContext(ctx).Select("id, name, user_id").Find(&coaches)
	coachNameOfUser := map[int64]string{}
	for _, c := range coaches {
		coachNames[c.ID] = c.Name
		if c.UserID > 0 {
			coachNameOfUser[c.UserID] = c.Name
		}
	}
	nicknames := map[int64]string{}
	if len(uids) > 0 {
		ids := make([]int64, 0, len(uids))
		for id := range uids {
			ids = append(ids, id)
		}
		var users []model.User
		s.db.WithContext(ctx).Select("id, nickname").Where("id IN ?", ids).Find(&users)
		for _, u := range users {
			nicknames[u.ID] = u.Nickname
		}
	}
	nameOf := func(uid int64) string {
		if n, ok := coachNameOfUser[uid]; ok && n != "" {
			return n
		}
		return nicknames[uid]
	}
	out := make([]map[string]interface{}, 0, len(qs))
	for _, q := range qs {
		asker := nameOf(q.UserID)
		if q.Anonymous == 1 && !realName {
			asker = "匿名学员"
		}
		list := answersOf[q.ID]
		if list == nil {
			list = []map[string]interface{}{}
		}
		for _, a := range list {
			a["authorName"] = nameOf(a["userId"].(int64))
		}
		out = append(out, map[string]interface{}{
			"id": q.ID, "content": q.Content, "status": q.Status, "pinned": q.Pinned,
			"createdAt": q.CreatedAt, "asker": asker, "userId": q.UserID, "answers": list,
		})
	}
	return out
}

// ListOfPackage 课包公开问答（visible；置顶优先、时间倒序）
func (s *QaService) ListOfPackage(ctx context.Context, packageID int64, page, pageSize int) ([]map[string]interface{}, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	q := s.db.WithContext(ctx).Model(&model.QaQuestion{}).
		Where("package_id = ? AND status = ?", packageID, model.QaStatusVisible)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var qs []model.QaQuestion
	if err := q.Order("pinned DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&qs).Error; err != nil {
		return nil, 0, err
	}
	return s.enrichQuestions(ctx, qs, false, false), total, nil
}

// MyQuestions 我的提问（全部状态，含隐藏标记）
func (s *QaService) MyQuestions(ctx context.Context, userID int64) ([]map[string]interface{}, error) {
	var qs []model.QaQuestion
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("id DESC").Limit(100).Find(&qs).Error; err != nil {
		return nil, err
	}
	out := s.enrichQuestions(ctx, qs, false, true)
	pids := map[int64]bool{}
	for _, q := range qs {
		pids[q.PackageID] = true
	}
	titles := map[int64]string{}
	if len(pids) > 0 {
		ids := make([]int64, 0, len(pids))
		for id := range pids {
			ids = append(ids, id)
		}
		var pkgs []model.CoursePackage
		s.db.WithContext(ctx).Select("id, title").Where("id IN ?", ids).Find(&pkgs)
		for _, p := range pkgs {
			titles[p.ID] = p.Title
		}
	}
	for i, item := range out {
		q := qs[i]
		item["packageTitle"] = titles[q.PackageID]
	}
	return out, nil
}

// CoachQuestions 教练端：名下课包的问题（含隐藏状态展示；提问者真实昵称）
func (s *QaService) CoachQuestions(ctx context.Context, coachID int64) ([]map[string]interface{}, error) {
	var qs []model.QaQuestion
	err := s.db.WithContext(ctx).
		Joins("JOIN coach_package cp ON cp.package_id = qa_question.package_id").
		Where("cp.coach_id = ?", coachID).
		Order("qa_question.pinned DESC, qa_question.id DESC").Limit(100).
		Find(&qs).Error
	if err != nil {
		return nil, err
	}
	return s.enrichQuestions(ctx, qs, false, true), nil
}

// TogglePinCoach 教师置顶/取消置顶自己课包的问题（沉淀 FAQ）
func (s *QaService) TogglePinCoach(ctx context.Context, coachID, questionID int64) error {
	var q model.QaQuestion
	if err := s.db.WithContext(ctx).First(&q, questionID).Error; err != nil {
		return errs.NotFound("问题不存在")
	}
	var cnt int64
	s.db.WithContext(ctx).Model(&model.CoachPackage{}).
		Where("coach_id = ? AND package_id = ?", coachID, q.PackageID).Count(&cnt)
	if cnt == 0 {
		return errs.Forbidden("只能置顶自己课包的问题")
	}
	val := 1 - q.Pinned
	return s.db.WithContext(ctx).Model(&model.QaQuestion{}).
		Where("id = ?", questionID).Update("pinned", val).Error
}

// ---- 后台管理 ----

// AdminList 后台问答列表（状态/关键词筛选）
func (s *QaService) AdminList(ctx context.Context, status, keyword string, page, pageSize int) ([]map[string]interface{}, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	q := s.db.WithContext(ctx).Model(&model.QaQuestion{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if keyword != "" {
		q = q.Where("content LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var qs []model.QaQuestion
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&qs).Error; err != nil {
		return nil, 0, err
	}
	out := s.enrichQuestions(ctx, qs, true, true)
	// 附课包标题
	for i, item := range out {
		var pkg model.CoursePackage
		if err := s.db.WithContext(ctx).Select("title").First(&pkg, qs[i].PackageID).Error; err == nil {
			item["packageTitle"] = pkg.Title
		}
	}
	return out, total, nil
}

// SetQuestionStatus 后台隐藏/恢复问题
func (s *QaService) SetQuestionStatus(ctx context.Context, questionID int64, status string) error {
	if status != model.QaStatusVisible && status != model.QaStatusHidden {
		return errs.BadParam("status 仅支持 visible/hidden")
	}
	res := s.db.WithContext(ctx).Model(&model.QaQuestion{}).
		Where("id = ?", questionID).Update("status", status)
	if res.RowsAffected == 0 {
		return errs.NotFound("问题不存在")
	}
	return res.Error
}

// SetAnswerStatus 后台隐藏/恢复回答
func (s *QaService) SetAnswerStatus(ctx context.Context, answerID int64, status string) error {
	if status != model.QaStatusVisible && status != model.QaStatusHidden {
		return errs.BadParam("status 仅支持 visible/hidden")
	}
	res := s.db.WithContext(ctx).Model(&model.QaAnswer{}).
		Where("id = ?", answerID).Update("status", status)
	if res.RowsAffected == 0 {
		return errs.NotFound("回答不存在")
	}
	return res.Error
}

// AdminPinQuestion 后台置顶/取消置顶
func (s *QaService) AdminPinQuestion(ctx context.Context, questionID int64, pinned bool) error {
	val := int8(0)
	if pinned {
		val = 1
	}
	res := s.db.WithContext(ctx).Model(&model.QaQuestion{}).
		Where("id = ?", questionID).Update("pinned", val)
	if res.RowsAffected == 0 {
		return errs.NotFound("问题不存在")
	}
	return res.Error
}
