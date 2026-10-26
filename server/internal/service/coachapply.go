package service

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
)

// CoachApplicationService 教练入驻：小程序端申请 → 后台审核（RequireBoss）→
// 通过自动建档并绑定申请人账号（登录即 isCoach）+ 通知结果。
// 同时修复安全一致性：教练自改资质图会重置 cert_verified（重新审核）。
type CoachApplicationService struct {
	db             *gorm.DB
	notify         *NotifyService // 可为 nil
	uploadPrefixes []string
}

func NewCoachApplicationService(db *gorm.DB) *CoachApplicationService {
	return &CoachApplicationService{db: db, uploadPrefixes: []string{"/uploads/"}}
}

func (s *CoachApplicationService) WithNotify(n *NotifyService) *CoachApplicationService {
	s.notify = n
	return s
}

// WithUploadPrefixes 图片合法前缀（路由装配传入 upload.base_url）
func (s *CoachApplicationService) WithUploadPrefixes(ps ...string) *CoachApplicationService {
	for _, p := range ps {
		if p != "" {
			s.uploadPrefixes = append(s.uploadPrefixes, p)
		}
	}
	return s
}

// Submit 提交入驻申请
func (s *CoachApplicationService) Submit(ctx context.Context, userID int64, req *model.CoachApplication) (*model.CoachApplication, error) {
	// 已是在职教练不可重复申请
	var coach model.Coach
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND status = 1", userID).First(&coach).Error; err == nil {
		return nil, errs.Biz("您已是教练，无需申请")
	}
	// 已有待审核申请不可重复提交
	var exist model.CoachApplication
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, model.CoachApplyPending).
		First(&exist).Error; err == nil {
		return nil, errs.Biz("已有待审核的申请，请耐心等待")
	}
	name := strings.TrimSpace(req.Name)
	if len(name) < 2 || len(name) > 64 {
		return nil, errs.BadParam("姓名需 2~64 字")
	}
	if req.Years < 0 || req.Years > 50 {
		return nil, errs.BadParam("从业年限需在 0~50 年")
	}
	if len(req.Intro) > 1000 {
		return nil, errs.BadParam("自我介绍最多 1000 字")
	}
	if len(strings.TrimSpace(req.Phone)) > 20 {
		return nil, errs.BadParam("联系方式过长")
	}
	certJSON, err := sanitizeCertImages(req.CertImages, s.uploadPrefixes)
	if err != nil {
		return nil, err
	}
	rec := model.CoachApplication{
		OwnerID: 1, UserID: userID,
		Name: name, Gender: req.Gender, Years: req.Years,
		Specialties: strings.TrimSpace(req.Specialties), CertImages: certJSON,
		Intro: req.Intro, Phone: strings.TrimSpace(req.Phone),
		Status: model.CoachApplyPending,
	}
	if err := s.db.WithContext(ctx).Create(&rec).Error; err != nil {
		return nil, errs.Server(err, "提交申请失败")
	}
	return &rec, nil
}

// Mine 我最新一次申请（含驳回原因）
func (s *CoachApplicationService) Mine(ctx context.Context, userID int64) (map[string]interface{}, error) {
	out := map[string]interface{}{"exists": false}
	var rec model.CoachApplication
	if err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).Order("id DESC").First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return out, nil
		}
		return nil, err
	}
	out["exists"] = true
	out["application"] = rec
	return out, nil
}

// AdminList 后台申请列表（status=0 待审）
func (s *CoachApplicationService) AdminList(ctx context.Context, status int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.CoachApplication{})
	if status >= 0 {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var recs []model.CoachApplication
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&recs).Error; err != nil {
		return nil, 0, err
	}
	// 附申请人昵称
	ids := make([]int64, 0, len(recs))
	for _, r := range recs {
		ids = append(ids, r.UserID)
	}
	names := map[int64]string{}
	if len(ids) > 0 {
		var users []model.User
		if err := s.db.WithContext(ctx).Select("id, nickname").Where("id IN ?", ids).Find(&users).Error; err == nil {
			for _, u := range users {
				names[u.ID] = u.Nickname
			}
		}
	}
	out := make([]map[string]interface{}, 0, len(recs))
	for _, r := range recs {
		out = append(out, map[string]interface{}{
			"application": r, "userNickname": names[r.UserID],
		})
	}
	return out, total, nil
}

// Review 审核：approve=true 通过（pending→approved 条件更新幂等闸门 → 同事务内 upsert coach
// 并绑定账号，P1-18：建档失败整体回滚，申请保持 pending 可重审）→ 事务外通知结果；
// false 驳回（留痕 + 通知）。
func (s *CoachApplicationService) Review(ctx context.Context, adminID, applyID int64, approve bool, reason string) (*model.CoachApplication, error) {
	var rec model.CoachApplication
	if err := s.db.WithContext(ctx).First(&rec, applyID).Error; err != nil {
		return nil, errs.NotFound("申请不存在")
	}
	newStatus := int8(model.CoachApplyRejected)
	if approve {
		newStatus = model.CoachApplyApproved
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 幂等闸门：pending → 已审
		res := tx.Model(&model.CoachApplication{}).
			Where("id = ? AND status = ?", applyID, model.CoachApplyPending).
			Updates(map[string]interface{}{"status": newStatus, "reject_reason": reason, "admin_id": adminID})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errs.Conflict("该申请已被处理")
		}
		if approve {
			return upsertCoachTx(tx, &rec)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	rec.Status = newStatus
	rec.RejectReason = reason
	// 事务外通知申请人（失败仅记日志）
	if s.notify != nil {
		text := "很遗憾，您的教练入驻申请未通过"
		if approve {
			text = "您的教练入驻申请已通过，欢迎加入！"
		}
		if reason != "" {
			text += "：" + reason
		}
		if _, err := s.notify.Send(ctx, rec.UserID, model.NotifySceneCoachApplyResult, rec.ID,
			map[string]string{"thing1": text}); err != nil {
			log.Printf("[coachapply] 通知发送失败 user=%d: %v", rec.UserID, err)
		}
	}
	return &rec, nil
}

// upsertCoachTx 通过后建档/更新并绑定申请人账号（由审核事务传入 tx，P1-18）
func upsertCoachTx(tx *gorm.DB, rec *model.CoachApplication) error {
	var coach model.Coach
	err := tx.Where("user_id = ?", rec.UserID).First(&coach).Error
	switch {
	case err == nil:
		// 曾绑定过的账号：更新资料并恢复在职（停用教练重新申请的路径）
		coach.Name, coach.Gender, coach.Years = rec.Name, rec.Gender, rec.Years
		coach.Specialties, coach.CertImages, coach.Intro = rec.Specialties, rec.CertImages, rec.Intro
		coach.CertVerified, coach.Status = 1, 1
		if err := tx.Save(&coach).Error; err != nil {
			return err
		}
	case err == gorm.ErrRecordNotFound:
		coach = model.Coach{
			OwnerID: rec.OwnerID, UserID: rec.UserID,
			Name: rec.Name, Gender: rec.Gender, Years: rec.Years,
			Specialties: rec.Specialties, CertImages: rec.CertImages, Intro: rec.Intro,
			CertVerified: 1, Status: 1,
		}
		if err := tx.Create(&coach).Error; err != nil {
			return err
		}
	default:
		return errs.Server(err, "查询教练档案失败")
	}
	return nil
}

// sanitizeCertImages 资质图校验：1~6 张、必须站内上传前缀
func sanitizeCertImages(imagesJSON string, prefixes []string) (string, error) {
	if strings.TrimSpace(imagesJSON) == "" {
		return "", errs.BadParam("请上传至少 1 张资质证书图片")
	}
	var images []string
	if err := json.Unmarshal([]byte(imagesJSON), &images); err != nil {
		return "", errs.BadParam("资质图片数据异常")
	}
	if len(images) == 0 {
		return "", errs.BadParam("请上传至少 1 张资质证书图片")
	}
	if len(images) > 6 {
		return "", errs.BadParam("资质图片最多 6 张")
	}
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
			return "", errs.BadParam("资质图片必须来自站内上传")
		}
	}
	b, err := json.Marshal(images)
	if err != nil {
		return "", errs.Server(err, "图片数据异常")
	}
	return string(b), nil
}
