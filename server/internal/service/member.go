package service

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
)

// MemberService 学员课时账户查询与后台手动调整。
type MemberService struct{ db *gorm.DB }

func NewMemberService(db *gorm.DB) *MemberService { return &MemberService{db: db} }

// MyPackages 我的课包（含剩余课时与状态）
func (s *MemberService) MyPackages(ctx context.Context, userID int64) ([]model.MemberPackage, error) {
	var rows []model.MemberPackage
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("CASE status WHEN 'active' THEN 1 WHEN 'inactive' THEN 2 WHEN 'exhausted' THEN 3 WHEN 'expired' THEN 4 ELSE 5 END, id DESC").Find(&rows).Error
	return rows, err
}

// MyTransactions 某课包的课时流水
func (s *MemberService) MyTransactions(ctx context.Context, userID, memberPackageID int64) ([]model.PackageTransaction, error) {
	var mp model.MemberPackage
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", memberPackageID, userID).
		First(&mp).Error; err != nil {
		return nil, errs.NotFound("课包不存在")
	}
	var rows []model.PackageTransaction
	err := s.db.WithContext(ctx).Where("member_package_id = ?", memberPackageID).
		Order("id DESC").Limit(100).Find(&rows).Error
	return rows, err
}

// AdjustSessions 后台手动调整（赠课 +N / 扣课 -N）。
// 每次操作生成独立流水；正数加 total、负数计 used，剩余课时始终 = total - used。
// clientKey 非空时作为幂等键（后台防双击/重放，同键重复提交直接拒绝）；
// 为空则生成随机键（历史行为：每次调用都是一笔独立调整）。
func (s *MemberService) AdjustSessions(ctx context.Context, userID, memberPackageID int64, change int, remark, adminName, clientKey string) (*model.MemberPackage, error) {
	if change == 0 {
		return nil, errs.BadParam("调整数量不能为 0")
	}
	key := "adjust:" + randHex(12)
	if strings.TrimSpace(clientKey) != "" {
		key = "adjust:client:" + strings.TrimSpace(clientKey)
	}
	var mp model.MemberPackage
	err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", memberPackageID, userID).First(&mp).Error
	if err != nil {
		return nil, errs.NotFound("学员课包不存在")
	}
	if mp.Status == model.MPStatusRefunded {
		return nil, errs.Biz("该课包已退款回收，不可调整")
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model.PackageTransaction{
			MemberPackageID: mp.ID, UserID: userID, SessionsChange: change,
			Type: model.TxnTypeManualAdjust,
			IdempotencyKey: key,
			BizType: "manual", BizID: 0, Remark: "[" + adminName + "] " + remark,
		}).Error; err != nil {
			if isDuplicateKey(err) {
				return errs.Biz("重复的调整请求，请勿重复提交")
			}
			return err
		}
		updates := map[string]interface{}{"updated_at": time.Now()}
		if change > 0 {
			updates["total_sessions"] = gorm.Expr("total_sessions + ?", change)
		} else {
			// 扣课：used 上限不超过 total（CASE 写法兼容 MySQL 与 sqlite）
			updates["used_sessions"] = gorm.Expr(
				"CASE WHEN used_sessions + ? > total_sessions THEN total_sessions ELSE used_sessions + ? END", -change, -change)
		}
		return tx.Model(&model.MemberPackage{}).Where("id = ?", mp.ID).Updates(updates).Error
	})
	if err != nil {
		if ae, ok := err.(*errs.AppError); ok {
			return nil, ae
		}
		return nil, errs.Server(err, "调整失败")
	}
	var updated model.MemberPackage
	_ = s.db.WithContext(ctx).First(&updated, mp.ID).Error
	return &updated, nil
}

// ListMembersAdmin 后台学员列表（含课包汇总）
func (s *MemberService) ListMembersAdmin(ctx context.Context, keyword string, page, pageSize int) ([]map[string]interface{}, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	q := s.db.WithContext(ctx).Model(&model.User{})
	if keyword != "" {
		q = q.Where("nickname LIKE ? OR phone LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []model.User
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	out := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		var pkgCount int64
		var remaining int64
		s.db.WithContext(ctx).Model(&model.MemberPackage{}).
			Where("user_id = ? AND status IN ?", u.ID, []string{model.MPStatusActive, model.MPStatusInactive}).
			Count(&pkgCount)
		s.db.WithContext(ctx).Model(&model.MemberPackage{}).
			Where("user_id = ? AND status IN ?", u.ID, []string{model.MPStatusActive, model.MPStatusInactive}).
			Select("COALESCE(SUM(total_sessions - used_sessions),0)").Scan(&remaining)
		var orderCount int64
		s.db.WithContext(ctx).Model(&model.Order{}).
			Where("user_id = ? AND status = ?", u.ID, model.OrderStatusPaid).Count(&orderCount)
		out = append(out, map[string]interface{}{
			"user": u, "packageCount": pkgCount, "remainingSessions": remaining, "paidOrderCount": orderCount,
		})
	}
	return out, total, nil
}

// MemberPackagesAdmin 后台查某学员全部课包
func (s *MemberService) MemberPackagesAdmin(ctx context.Context, userID int64) ([]model.MemberPackage, error) {
	var rows []model.MemberPackage
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("id DESC").Find(&rows).Error
	return rows, err
}
