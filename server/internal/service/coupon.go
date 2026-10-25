package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
)

// CouponService 优惠券底座（规划 §7.1）。
// 发放渠道：领券中心自领（Claim）/ 后台定向发（Grant，不限每人张数）/ 新人礼包自动发放。
// 券模板行上的条件 UPDATE（issued_count+1）同时充当行锁：同模板的所有发放在事务内串行，
// 总量防超发、每人限领的计数检查因此无并发窗口。
// 用券一律经 ApplyCouponForOrder 在订单事务内核销（usable→used 绑定订单，条件更新幂等）。
type CouponService struct {
	db *gorm.DB
}

func NewCouponService(db *gorm.DB) *CouponService { return &CouponService{db: db} }

// DB 暴露底层连接（后台列表/统计查询用）
func (s *CouponService) DB() *gorm.DB { return s.db }

// FenYuan 分转元展示（标签用）
func FenYuan(fen int64) string {
	return fmt.Sprintf("%.2f", float64(fen)/100)
}

// TemplateLabel 券展示文案：满100减20 / 立减20 / 8.8折（限指定课包时附说明）
func TemplateLabel(t *model.CouponTemplate) string {
	var label string
	switch t.Type {
	case model.CouponTypeDiscount:
		label = fmt.Sprintf("%s折", strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", float64(t.Amount)/100), "0"), "."))
	default:
		if t.Threshold > 0 {
			label = fmt.Sprintf("满%s减%s元", FenYuan(t.Threshold), FenYuan(t.Amount))
		} else {
			label = fmt.Sprintf("立减%s元", FenYuan(t.Amount))
		}
	}
	return label
}

// CouponBenefit 计算优惠额（分）。门槛按传入价格判断，优惠封顶 price-1（微信要求实付>0）。
func CouponBenefit(t *model.CouponTemplate, price int64) int64 {
	if t.Threshold > 0 && price < t.Threshold {
		return 0
	}
	var b int64
	switch t.Type {
	case model.CouponTypeDiscount:
		b = price * (10000 - t.Amount) / 10000
	case model.CouponTypeReduce:
		b = t.Amount
	}
	if b > price-1 {
		b = price - 1
	}
	if b < 0 {
		b = 0
	}
	return b
}

// Center 领券中心：在售模板 + 我的已领次数 + 是否可领
func (s *CouponService) Center(ctx context.Context, userID int64) ([]map[string]interface{}, error) {
	var tpls []model.CouponTemplate
	if err := s.db.WithContext(ctx).
		Where("status = 1 AND (total_count = 0 OR issued_count < total_count)").
		Order("id DESC").Limit(50).Find(&tpls).Error; err != nil {
		return nil, err
	}
	type row struct {
		TemplateID int64
		C          int64
	}
	var counts []row
	s.db.WithContext(ctx).Model(&model.UserCoupon{}).
		Select("template_id, COUNT(*) AS c").
		Where("user_id = ?", userID).Group("template_id").Scan(&counts)
	mine := map[int64]int64{}
	for _, r := range counts {
		mine[r.TemplateID] = r.C
	}
	out := make([]map[string]interface{}, 0, len(tpls))
	for i := range tpls {
		t := &tpls[i]
		got := mine[t.ID]
		out = append(out, map[string]interface{}{
			"templateId": t.ID, "name": t.Name, "type": t.Type, "label": TemplateLabel(t),
			"threshold": t.Threshold, "amount": t.Amount, "packageId": t.PackageID,
			"validDays": t.ValidDays, "remark": t.Remark,
			"myReceived": got, "claimable": got < int64(t.PerUserLimit),
		})
	}
	return out, nil
}

// Claim 领券中心自领（受总量与每人限领约束）
func (s *CouponService) Claim(ctx context.Context, userID, templateID int64) (*model.UserCoupon, error) {
	return s.issue(ctx, userID, templateID, false)
}

// Grant 后台定向发放（绕过每人限领，仍受总量约束）
func (s *CouponService) Grant(ctx context.Context, userID, templateID int64) (*model.UserCoupon, error) {
	return s.issue(ctx, userID, templateID, true)
}

func (s *CouponService) issue(ctx context.Context, userID, templateID int64, ignoreUserLimit bool) (*model.UserCoupon, error) {
	var out *model.UserCoupon
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 条件更新 = 抢行锁 + 总量校验，同模板的发放在此串行
		res := tx.Model(&model.CouponTemplate{}).
			Where("id = ? AND status = 1 AND (total_count = 0 OR issued_count < total_count)", templateID).
			UpdateColumn("issued_count", gorm.Expr("issued_count + 1"))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errs.Biz("该券已抢完或已停发")
		}
		var tpl model.CouponTemplate
		if err := tx.First(&tpl, templateID).Error; err != nil {
			return errs.NotFound("券不存在")
		}
		if !ignoreUserLimit {
			var mine int64
			if err := tx.Model(&model.UserCoupon{}).
				Where("user_id = ? AND template_id = ?", userID, templateID).Count(&mine).Error; err != nil {
				return err
			}
			if mine >= int64(tpl.PerUserLimit) {
				return errs.Biz("该券每人限领 " + fmt.Sprint(tpl.PerUserLimit) + " 张")
			}
		}
		exp := time.Now().AddDate(0, 0, tpl.ValidDays)
		uc := model.UserCoupon{UserID: userID, TemplateID: templateID, Status: model.CouponUsable, ExpireAt: &exp}
		if err := tx.Create(&uc).Error; err != nil {
			return err
		}
		out = &uc
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// expireMineLazy 读前提级：我的可用券中已过期的置 expired（任务每小时兜底全量）
func (s *CouponService) expireMineLazy(ctx context.Context, userID int64) {
	s.db.WithContext(ctx).Model(&model.UserCoupon{}).
		Where("user_id = ? AND status = ? AND expire_at <= ?", userID, model.CouponUsable, time.Now()).
		Update("status", model.CouponExpired)
}

// Mine 我的券包（附带模板信息）
func (s *CouponService) Mine(ctx context.Context, userID int64, status string) ([]map[string]interface{}, error) {
	s.expireMineLazy(ctx, userID)
	q := s.db.WithContext(ctx).Model(&model.UserCoupon{}).Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var ucs []model.UserCoupon
	if err := q.Order("status = 'usable' DESC, id DESC").Limit(100).Find(&ucs).Error; err != nil {
		return nil, err
	}
	return s.enrich(ctx, ucs), nil
}

// UsableForSku 某 SKU 下我的可用券（范围/门槛过滤），附优惠额与折后价
func (s *CouponService) UsableForSku(ctx context.Context, userID, skuID int64) ([]map[string]interface{}, error) {
	var sku model.PackageSKU
	if err := s.db.WithContext(ctx).Where("id = ? AND status = 1", skuID).First(&sku).Error; err != nil {
		return nil, errs.NotFound("课包规格不存在或已停售")
	}
	s.expireMineLazy(ctx, userID)
	var ucs []model.UserCoupon
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND status = ? AND expire_at > ?",
			userID, model.CouponUsable, time.Now()).
		Order("id DESC").Limit(100).Find(&ucs).Error; err != nil {
		return nil, err
	}
	enriched := s.enrich(ctx, ucs)
	out := make([]map[string]interface{}, 0, len(enriched))
	for _, it := range enriched {
		tpl := it["_tpl"].(*model.CouponTemplate)
		if tpl.PackageID != 0 && tpl.PackageID != sku.PackageID {
			continue
		}
		benefit := CouponBenefit(tpl, sku.Price)
		if benefit <= 0 {
			continue // 未达门槛
		}
		delete(it, "_tpl")
		it["benefit"] = benefit
		it["finalPay"] = sku.Price - benefit
		out = append(out, it)
	}
	return out, nil
}

// enrich 补模板信息（_tpl 供内部继续计算，输出前删除）
func (s *CouponService) enrich(ctx context.Context, ucs []model.UserCoupon) []map[string]interface{} {
	tids := map[int64]bool{}
	for _, uc := range ucs {
		tids[uc.TemplateID] = true
	}
	tpls := map[int64]*model.CouponTemplate{}
	if len(tids) > 0 {
		ids := make([]int64, 0, len(tids))
		for id := range tids {
			ids = append(ids, id)
		}
		var ts []model.CouponTemplate
		s.db.WithContext(ctx).Where("id IN ?", ids).Find(&ts)
		for i := range ts {
			tpls[ts[i].ID] = &ts[i]
		}
	}
	out := make([]map[string]interface{}, 0, len(ucs))
	for _, uc := range ucs {
		tpl := tpls[uc.TemplateID]
		it := map[string]interface{}{
			"couponId": uc.ID, "templateId": uc.TemplateID, "status": uc.Status,
			"orderId": uc.OrderID, "expireAt": uc.ExpireAt, "usedAt": uc.UsedAt,
			"_tpl": tpl,
		}
		if tpl != nil {
			it["name"] = tpl.Name
			it["label"] = TemplateLabel(tpl)
			it["packageId"] = tpl.PackageID
		}
		out = append(out, it)
	}
	return out
}

// ApplyCouponForOrder 订单事务内核销券：usable→used 绑定订单（条件更新幂等），
// 返回实付优惠额。所有校验以事务内数据为准，任何不满足即整体回滚（订单不落库）。
func ApplyCouponForOrder(tx *gorm.DB, userID, couponID, orderID, packageID, skuPrice int64) (int64, error) {
	var uc model.UserCoupon
	if err := tx.Where("id = ? AND user_id = ?", couponID, userID).First(&uc).Error; err != nil {
		return 0, errs.Biz("优惠券不存在")
	}
	var tpl model.CouponTemplate
	if err := tx.First(&tpl, uc.TemplateID).Error; err != nil {
		return 0, errs.Biz("优惠券信息异常")
	}
	if tpl.PackageID != 0 && tpl.PackageID != packageID {
		return 0, errs.Biz("该优惠券不适用于此课包")
	}
	benefit := CouponBenefit(&tpl, skuPrice)
	if benefit <= 0 {
		return 0, errs.Biz("订单金额未满足用券条件")
	}
	res := tx.Model(&model.UserCoupon{}).
		Where("id = ? AND user_id = ? AND status = ? AND expire_at > ?",
			couponID, userID, model.CouponUsable, time.Now()).
		Updates(map[string]interface{}{
			"status": model.CouponUsed, "order_id": orderID, "used_at": time.Now(),
		})
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 {
		return 0, errs.Biz("优惠券不可用或已过期")
	}
	return benefit, nil
}

// returnCouponOnRefund 退款终态返还券：仍在有效期 → usable 可再用；已过期 → expired（留痕）。
// 拒绝退款不调用（订单回 paid，券保持 used 语义正确）。
func returnCouponOnRefund(tx *gorm.DB, order *model.Order) error {
	if order.CouponID == 0 {
		return nil
	}
	now := time.Now()
	if err := tx.Model(&model.UserCoupon{}).
		Where("id = ? AND order_id = ? AND status = ? AND expire_at > ?",
			order.CouponID, order.ID, model.CouponUsed, now).
		Update("status", model.CouponUsable).Error; err != nil {
		return err
	}
	return tx.Model(&model.UserCoupon{}).
		Where("id = ? AND order_id = ? AND status = ?",
			order.CouponID, order.ID, model.CouponUsed).
		Update("status", model.CouponExpired).Error
}

// ValidateTemplate 后台保存券模板的校验
func ValidateTemplate(t *model.CouponTemplate) error {
	if strings.TrimSpace(t.Name) == "" {
		return errs.BadParam("券名称不能为空")
	}
	if t.Type != model.CouponTypeReduce && t.Type != model.CouponTypeDiscount {
		return errs.BadParam("券类型仅支持 reduce/discount")
	}
	if t.Threshold < 0 {
		return errs.BadParam("用券门槛不能为负数")
	}
	if t.Type == model.CouponTypeReduce && (t.Amount <= 0 || t.Amount > 10_000_000) {
		return errs.BadParam("满减券金额需在 1 分 ~ 10 万元之间")
	}
	if t.Type == model.CouponTypeDiscount && (t.Amount < 1000 || t.Amount > 9900) {
		return errs.BadParam("折扣率需在 9 折 ~ 1 折之间（1000~9900 万分比）")
	}
	if t.ValidDays <= 0 || t.ValidDays > 3650 {
		return errs.BadParam("券有效期需在 1 ~ 3650 天之间")
	}
	if t.PerUserLimit <= 0 || t.PerUserLimit > 100 {
		return errs.BadParam("每人限领需在 1 ~ 100 张之间")
	}
	if t.TotalCount < 0 {
		return errs.BadParam("发放总量不能为负数")
	}
	return nil
}
