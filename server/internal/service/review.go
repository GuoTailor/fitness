package service

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
)

// ReviewService 课后评价。提交门槛：本人已核销（verification=verified），
// 一次提交落双维度两条记录（课包评价 + 教练评价），
// uk_review(verification_id, target_type) 防重复提交。
// 默认待审核，后台通过后对外可见；教练维度通过/隐藏时刷新 coach 评分聚合。
type ReviewService struct {
	db             *gorm.DB
	cfg            *ConfigService
	uploadPrefixes []string          // 图片链接合法前缀（本站上传），默认 /uploads/
	coupons        *CouponService    // 评价奖励发券（配置 marketing.review_reward_template）
}

func NewReviewService(db *gorm.DB, cfg *ConfigService) *ReviewService {
	return &ReviewService{db: db, cfg: cfg, uploadPrefixes: []string{"/uploads/"}}
}

// WithUploadPrefixes 设置图片合法前缀（路由装配时传入 upload.base_url）
func (s *ReviewService) WithUploadPrefixes(ps ...string) *ReviewService {
	for _, p := range ps {
		if p != "" {
			s.uploadPrefixes = append(s.uploadPrefixes, p)
		}
	}
	return s
}

// WithRewardCoupons 装配评价奖励发券
func (s *ReviewService) WithRewardCoupons(c *CouponService) *ReviewService {
	s.coupons = c
	return s
}

// rewardForReview 评价奖励发券：配置了奖励模板且评价通过后发放；
// 未配置/超限/停发一律静默跳过（记日志），绝不影响评价主流程。
// 重复发放由券模板的每人限领天然控制。
func (s *ReviewService) rewardForReview(ctx context.Context, userID int64) {
	if s.coupons == nil || s.cfg == nil {
		return
	}
	idStr := strings.TrimSpace(s.cfg.Get(ctx, model.CfgReviewRewardTemplate, ""))
	if idStr == "" {
		return
	}
	tid, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || tid <= 0 {
		return
	}
	if _, err := s.coupons.Claim(ctx, userID, tid); err != nil {
		log.Printf("[review] 评价奖励发券 user=%d tpl=%d 跳过: %v", userID, tid, err)
	}
}

// Reviewable 待评价列表：已核销且未评价（近 50 条），带课程名/教练名。
func (s *ReviewService) Reviewable(ctx context.Context, userID int64) ([]map[string]interface{}, error) {
	var vs []model.Verification
	err := s.db.WithContext(ctx).Raw(
		"SELECT v.* FROM verification v LEFT JOIN review r ON r.verification_id = v.id "+
			"WHERE v.user_id = ? AND v.status = ? AND r.id IS NULL "+
			"ORDER BY v.verified_at DESC LIMIT 50",
		userID, model.VerifyStatusVerified).Scan(&vs).Error
	if err != nil {
		return nil, err
	}
	coachIDs := make([]int64, 0, len(vs))
	for _, v := range vs {
		coachIDs = append(coachIDs, v.CoachID)
	}
	coaches := map[int64]model.Coach{}
	if len(coachIDs) > 0 {
		var cs []model.Coach
		s.db.WithContext(ctx).Where("id IN ?", coachIDs).Find(&cs)
		for _, c := range cs {
			coaches[c.ID] = c
		}
	}
	out := make([]map[string]interface{}, 0, len(vs))
	for _, v := range vs {
		out = append(out, map[string]interface{}{
			"verificationId": v.ID,
			"bizType":        v.BizType,
			"packageId":      s.packageIDOf(ctx, &v),
			"coachId":        v.CoachID,
			"coachName":      coaches[v.CoachID].Name,
			"projectName":    s.projectNameOf(ctx, &v),
			"verifiedAt":     v.VerifiedAt,
		})
	}
	return out, nil
}

// packageIDOf 核销关联的课包（供评价落库/前端跳转）
func (s *ReviewService) packageIDOf(ctx context.Context, v *model.Verification) int64 {
	switch v.BizType {
	case "private":
		var b model.Booking
		if err := s.db.WithContext(ctx).Select("package_id").First(&b, v.BookingID).Error; err == nil {
			return b.PackageID
		}
	case "group":
		var ge model.GroupEnrollment
		if err := s.db.WithContext(ctx).First(&ge, v.EnrollmentID).Error; err == nil {
			var gc model.GroupClass
			if err := s.db.WithContext(ctx).Select("package_id").First(&gc, ge.GroupClassID).Error; err == nil {
				return gc.PackageID
			}
		}
	}
	return 0
}

// projectNameOf 课程展示名（私教取课包标题，团课取课程标题）
func (s *ReviewService) projectNameOf(ctx context.Context, v *model.Verification) string {
	switch v.BizType {
	case "private":
		var pkg model.CoursePackage
		pkgID := s.packageIDOf(ctx, v)
		if pkgID > 0 && s.db.WithContext(ctx).Select("title").First(&pkg, pkgID).Error == nil {
			return pkg.Title
		}
	case "group":
		var ge model.GroupEnrollment
		if err := s.db.WithContext(ctx).First(&ge, v.EnrollmentID).Error; err == nil {
			var gc model.GroupClass
			if err := s.db.WithContext(ctx).Select("title").First(&gc, ge.GroupClassID).Error; err == nil {
				return gc.Title
			}
		}
	}
	return "已完成课程"
}

// Submit 提交评价：双维度一次落库（同事务），重复提交返回业务错误。
// images 为本站上传返回的图片 URL（≤4 张，前缀校验防外链注入）。
func (s *ReviewService) Submit(ctx context.Context, userID, verificationID int64, packageRating, coachRating int, content string, images []string) error {
	if packageRating < 1 || packageRating > 5 || coachRating < 1 || coachRating > 5 {
		return errs.BadParam("评分需在 1-5 星之间")
	}
	runes := []rune(strings.TrimSpace(content))
	if len(runes) > 500 {
		runes = runes[:500]
	}
	content = string(runes)
	imagesJSON, err := sanitizeReviewImages(images, s.uploadPrefixes)
	if err != nil {
		return err
	}

	var v model.Verification
	if err := s.db.WithContext(ctx).First(&v, verificationID).Error; err != nil {
		return errs.NotFound("核销记录不存在")
	}
	if v.UserID != userID {
		return errs.NotFound("核销记录不存在") // 数据隔离：不暴露他人资源是否存在
	}
	if v.Status != model.VerifyStatusVerified {
		return errs.Biz("课程核销完成后才能评价")
	}
	if v.BizType != "private" && v.BizType != "group" {
		return errs.BadParam("未知的核销类型")
	}
	packageID := s.packageIDOf(ctx, &v)

	status := model.ReviewStatusPending
	if s.cfg != nil && s.cfg.GetInt(ctx, model.CfgReviewAutoApprove, 0) == 1 {
		status = model.ReviewStatusApproved
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rows := []model.Review{
			{
				UserID: userID, TargetType: model.ReviewTargetPackage, PackageID: packageID, CoachID: v.CoachID,
				VerificationID: verificationID, Rating: int8(packageRating), Content: content, Images: imagesJSON, Status: status,
			},
			{
				UserID: userID, TargetType: model.ReviewTargetCoach, PackageID: packageID, CoachID: v.CoachID,
				VerificationID: verificationID, Rating: int8(coachRating), Content: content, Images: imagesJSON, Status: status,
			},
		}
		for i := range rows {
			if err := tx.Create(&rows[i]).Error; err != nil {
				if isDuplicateKey(err) {
					return errs.Biz("该课程已评价过")
				}
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if status == model.ReviewStatusApproved && v.CoachID > 0 {
		s.RefreshCoachRating(ctx, v.CoachID)
	}
	if status == model.ReviewStatusApproved {
		s.rewardForReview(ctx, userID) // 免审模式：提交即通过，直接发奖励券
	}
	return nil
}

// ListApproved 某课包/教练的公开评价（仅 approved，精选优先），带用户昵称。
func (s *ReviewService) ListApproved(ctx context.Context, targetType string, targetID int64, page, pageSize int) ([]map[string]interface{}, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	q := s.db.WithContext(ctx).Model(&model.Review{}).
		Where("target_type = ? AND status = ?", targetType, model.ReviewStatusApproved)
	switch targetType {
	case model.ReviewTargetPackage:
		q = q.Where("package_id = ?", targetID)
	case model.ReviewTargetCoach:
		q = q.Where("coach_id = ?", targetID)
	default:
		return nil, 0, errs.BadParam("未知的评价维度")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var reviews []model.Review
	if err := q.Order("featured DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&reviews).Error; err != nil {
		return nil, 0, err
	}
	return s.enrichUsers(ctx, reviews), total, nil
}

// MyReviews 我的评价（全部状态）
func (s *ReviewService) MyReviews(ctx context.Context, userID int64) ([]map[string]interface{}, error) {
	var reviews []model.Review
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("id DESC").Limit(100).Find(&reviews).Error
	if err != nil {
		return nil, err
	}
	return s.enrichUsers(ctx, reviews), nil
}

// CoachReviews 教练收到的评价（含各状态，供教练回复）
func (s *ReviewService) CoachReviews(ctx context.Context, coachID int64) ([]map[string]interface{}, error) {
	var reviews []model.Review
	err := s.db.WithContext(ctx).
		Where("target_type = ? AND coach_id = ?", model.ReviewTargetCoach, coachID).
		Order("id DESC").Limit(100).Find(&reviews).Error
	if err != nil {
		return nil, err
	}
	return s.enrichUsers(ctx, reviews), nil
}

// Reply 教练回复评价（仅自己的教练维度评价）
func (s *ReviewService) Reply(ctx context.Context, coachID, reviewID int64, content string) error {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) == 0 {
		return errs.BadParam("回复内容不能为空")
	}
	if len(runes) > 200 {
		runes = runes[:200]
	}
	res := s.db.WithContext(ctx).Model(&model.Review{}).
		Where("id = ? AND target_type = ? AND coach_id = ?",
			reviewID, model.ReviewTargetCoach, coachID).
		Updates(map[string]interface{}{"coach_reply": string(runes), "coach_reply_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errs.NotFound("评价不存在")
	}
	return nil
}

// AdminList 后台评价列表（可按状态/维度筛选）
func (s *ReviewService) AdminList(ctx context.Context, status, targetType string, page, pageSize int) ([]map[string]interface{}, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	q := s.db.WithContext(ctx).Model(&model.Review{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if targetType != "" {
		q = q.Where("target_type = ?", targetType)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var reviews []model.Review
	if err := q.Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&reviews).Error; err != nil {
		return nil, 0, err
	}
	return s.enrichUsers(ctx, reviews), total, nil
}

// Audit 后台审核：action = approve / hide。教练维度变化后刷新评分聚合。
func (s *ReviewService) Audit(ctx context.Context, reviewID int64, action string) error {
	var r model.Review
	if err := s.db.WithContext(ctx).First(&r, reviewID).Error; err != nil {
		return errs.NotFound("评价不存在")
	}
	var status string
	switch action {
	case "approve":
		status = model.ReviewStatusApproved
	case "hide":
		status = model.ReviewStatusHidden
	default:
		return errs.BadParam("action 仅支持 approve/hide")
	}
	if err := s.db.WithContext(ctx).Model(&model.Review{}).
		Where("id = ?", reviewID).Update("status", status).Error; err != nil {
		return err
	}
	if r.TargetType == model.ReviewTargetCoach && r.CoachID > 0 {
		s.RefreshCoachRating(ctx, r.CoachID)
	}
	// 评价奖励：以课包维度审核通过为准（一次提交两条记录，避免双发）
	if action == "approve" && r.TargetType == model.ReviewTargetPackage {
		s.rewardForReview(ctx, r.UserID)
	}
	return nil
}

// SetFeatured 后台精选/取消精选
func (s *ReviewService) SetFeatured(ctx context.Context, reviewID int64, featured bool) error {
	var r model.Review
	if err := s.db.WithContext(ctx).First(&r, reviewID).Error; err != nil {
		return errs.NotFound("评价不存在")
	}
	val := int8(0)
	if featured {
		val = 1
	}
	return s.db.WithContext(ctx).Model(&model.Review{}).
		Where("id = ?", reviewID).Update("featured", val).Error
}

// RefreshCoachRating 重算教练评分聚合（approved 的教练维度评价）。
// 课包评分不落列，由详情页实时计算（shop.PackageDetail）。
func (s *ReviewService) RefreshCoachRating(ctx context.Context, coachID int64) {
	var avg float64
	var cnt int64
	s.db.WithContext(ctx).Model(&model.Review{}).
		Where("target_type = ? AND coach_id = ? AND status = ?",
			model.ReviewTargetCoach, coachID, model.ReviewStatusApproved).
		Select("COALESCE(AVG(rating),0), COUNT(*)").Row().Scan(&avg, &cnt)
	s.db.WithContext(ctx).Model(&model.Coach{}).Where("id = ?", coachID).
		Updates(map[string]interface{}{"rating_avg": avg, "rating_count": cnt})
}

// sanitizeReviewImages 评价图片校验：≤4 张、长度受限、必须以本站上传前缀开头
func sanitizeReviewImages(images []string, prefixes []string) (string, error) {
	if len(images) > 4 {
		return "", errs.BadParam("评价图片最多 4 张")
	}
	out := make([]string, 0, len(images))
	for _, u := range images {
		if len(u) > 512 {
			return "", errs.BadParam("图片链接异常")
		}
		ok := false
		for _, p := range prefixes {
			if p != "" && strings.HasPrefix(u, p) {
				ok = true
				break
			}
		}
		if !ok {
			return "", errs.BadParam("图片必须来自站内上传")
		}
		out = append(out, u)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", errs.Server(err, "图片数据异常")
	}
	return string(b), nil
}

// ParseReviewImages review.Images（JSON 数组字符串）→ 前端可用的数组
func ParseReviewImages(s string) []string {
	var out []string
	if s != "" {
		_ = json.Unmarshal([]byte(s), &out)
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// enrichUsers 评价列表补充昵称/头像
func (s *ReviewService) enrichUsers(ctx context.Context, reviews []model.Review) []map[string]interface{} {
	uids := map[int64]bool{}
	for _, r := range reviews {
		uids[r.UserID] = true
	}
	users := map[int64]model.User{}
	if len(uids) > 0 {
		ids := make([]int64, 0, len(uids))
		for id := range uids {
			ids = append(ids, id)
		}
		var us []model.User
		s.db.WithContext(ctx).Where("id IN ?", ids).Find(&us)
		for _, u := range us {
			users[u.ID] = u
		}
	}
	out := make([]map[string]interface{}, 0, len(reviews))
	for _, r := range reviews {
		item := map[string]interface{}{
			"review":     r,
			"nickname":   users[r.UserID].Nickname,
			"avatarUrl":  users[r.UserID].AvatarURL,
			"targetType": r.TargetType,
			"images":     ParseReviewImages(r.Images),
		}
		out = append(out, item)
	}
	return out
}
