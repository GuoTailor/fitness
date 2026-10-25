package service

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
)

// ---- 拼团限时活动（P3）----
// 生效期内开团用活动配置（价/人数/时长），到点自动失效回落 SKU 常开配置；
// 名额按开团数计，事务内条件 UPDATE 原子扣减防并发超开。

// ActiveActivityFor 取 SKU 当前生效的限时活动。
// 保存侧已保证同 SKU 时间不重叠，这里取最早开始者仅为查询兜底。
func ActiveActivityFor(ctx context.Context, db *gorm.DB, skuID int64) *model.GroupBuyActivity {
	now := time.Now()
	var act model.GroupBuyActivity
	if err := db.WithContext(ctx).
		Where("sku_id = ? AND status = 1 AND start_at <= ? AND end_at > ?", skuID, now, now).
		Order("start_at ASC, id ASC").First(&act).Error; err != nil {
		return nil
	}
	return &act
}

// claimActivity 活动名额原子扣减：窗口/上架状态/限额在一条 UPDATE 里判定，防并发超开
func claimActivity(tx *gorm.DB, act *model.GroupBuyActivity, now time.Time) error {
	res := tx.Model(&model.GroupBuyActivity{}).
		Where("id = ? AND status = 1 AND start_at <= ? AND end_at > ? AND (max_groups = 0 OR opened_count < max_groups)",
			act.ID, now, now).
		Update("opened_count", gorm.Expr("opened_count + 1"))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errs.Biz("拼团活动已结束或名额已满")
	}
	return nil
}

// ActivityInput 后台活动保存入参（时间由 handler 解析）
type ActivityInput struct {
	ID            int64
	Title         string
	SkuID         int64
	GroupPrice    int64
	GroupCapacity int
	StartAt       time.Time
	EndAt         time.Time
	MaxGroups     int
	ExpireHours   int
	Status        int8
}

// ActivitySave 新建/编辑活动（后台）。已有开团的活动禁改规格，防快照语义被事后篡改。
func (s *GroupBuyService) ActivitySave(ctx context.Context, in *ActivityInput) (*model.GroupBuyActivity, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return nil, errs.BadParam("请填写活动标题")
	}
	if in.GroupCapacity < 2 || in.GroupCapacity > 5 {
		return nil, errs.BadParam("成团人数需在 2~5 人")
	}
	if in.GroupPrice < 1 {
		return nil, errs.BadParam("拼团价需大于 0")
	}
	if in.StartAt.IsZero() || in.EndAt.IsZero() {
		return nil, errs.BadParam("请选择活动起止时间")
	}
	if !in.EndAt.After(in.StartAt) {
		return nil, errs.BadParam("结束时间需晚于开始时间")
	}
	if in.MaxGroups < 0 {
		in.MaxGroups = 0
	}
	if in.ExpireHours < 0 {
		in.ExpireHours = 0
	}

	var sku model.PackageSKU
	if err := s.db.WithContext(ctx).Where("id = ? AND status = 1", in.SkuID).First(&sku).Error; err != nil {
		return nil, errs.NotFound("课包规格不存在或已停售")
	}
	if in.GroupPrice >= sku.Price {
		return nil, errs.BadParam("拼团价需低于规格售价")
	}

	var pkg model.CoursePackage
	if err := s.db.WithContext(ctx).Select("id, title").First(&pkg, sku.PackageID).Error; err != nil {
		return nil, errs.NotFound("课包不存在或已下架")
	}

	act := model.GroupBuyActivity{
		Title: in.Title, SkuID: sku.ID, PackageID: sku.PackageID,
		GroupPrice: in.GroupPrice, GroupCapacity: in.GroupCapacity,
		StartAt: in.StartAt, EndAt: in.EndAt,
		MaxGroups: in.MaxGroups, ExpireHours: in.ExpireHours, Status: in.Status,
	}
	if in.ID > 0 {
		var exist model.GroupBuyActivity
		if err := s.db.WithContext(ctx).First(&exist, in.ID).Error; err != nil {
			return nil, errs.NotFound("活动不存在")
		}
		if exist.OpenedCount > 0 && exist.SkuID != act.SkuID {
			return nil, errs.BadParam("已有开团的活动不允许更换规格，请新建活动")
		}
		act.ID = exist.ID
		act.OwnerID = exist.OwnerID
		act.OpenedCount = exist.OpenedCount // 已开团数只增不减，由开团事务维护
	}
	// 同规格时间不重叠（不论上下架，排除自身），保证生效活动唯一
	var overlap int64
	if err := s.db.WithContext(ctx).Model(&model.GroupBuyActivity{}).
		Where("sku_id = ? AND id <> ? AND start_at < ? AND end_at > ?",
			act.SkuID, act.ID, act.EndAt, act.StartAt).
		Count(&overlap).Error; err != nil {
		return nil, err
	}
	if overlap > 0 {
		return nil, errs.Biz("该规格在此时间段已有拼团活动")
	}
	if err := s.db.WithContext(ctx).Save(&act).Error; err != nil {
		return nil, errs.Server(err, "保存拼团活动失败")
	}
	return &act, nil
}

// ActivityDelete 删除活动（仅未开过团的允许删，已有开团的走下架留痕）
func (s *GroupBuyService) ActivityDelete(ctx context.Context, id int64) error {
	var act model.GroupBuyActivity
	if err := s.db.WithContext(ctx).First(&act, id).Error; err != nil {
		return errs.NotFound("活动不存在")
	}
	if act.OpenedCount > 0 {
		return errs.Biz("活动已有开团，不能删除，请下架处理")
	}
	return s.db.WithContext(ctx).Delete(&model.GroupBuyActivity{}, id).Error
}

// ActivityList 后台活动列表（含已成团数与生效状态文案）
func (s *GroupBuyService) ActivityList(ctx context.Context) ([]map[string]interface{}, error) {
	var acts []model.GroupBuyActivity
	if err := s.db.WithContext(ctx).Order("id DESC").Limit(200).Find(&acts).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]map[string]interface{}, 0, len(acts))
	for i := range acts {
		a := &acts[i]
		var successCount int64
		s.db.WithContext(ctx).Model(&model.GroupBuy{}).
			Where("activity_id = ? AND status = ?", a.ID, model.GroupBuySuccess).
			Count(&successCount)
		pkgTitle, skuName := s.activityNames(ctx, a)
		out = append(out, map[string]interface{}{
			"id": a.ID, "title": a.Title, "skuId": a.SkuID, "packageId": a.PackageID,
			"pkgTitle": pkgTitle, "skuName": skuName,
			"groupPrice": a.GroupPrice, "groupCapacity": a.GroupCapacity,
			"startAt": a.StartAt, "endAt": a.EndAt,
			"maxGroups": a.MaxGroups, "openedCount": a.OpenedCount,
			"expireHours": a.ExpireHours, "status": a.Status,
			"successCount": successCount, "effectiveText": effectiveText(a, now),
			"createdAt": a.CreatedAt,
		})
	}
	return out, nil
}

// ActivityPublic Banner 落地信息（公开只读，仅暴露标题与价格）
func (s *GroupBuyService) ActivityPublic(ctx context.Context, id int64) (map[string]interface{}, error) {
	var act model.GroupBuyActivity
	if err := s.db.WithContext(ctx).Where("id = ? AND status = 1", id).First(&act).Error; err != nil {
		return nil, errs.NotFound("活动不存在或已下架")
	}
	pkgTitle, skuName := s.activityNames(ctx, &act)
	return map[string]interface{}{
		"id": act.ID, "title": act.Title, "skuId": act.SkuID, "packageId": act.PackageID,
		"pkgTitle": pkgTitle, "skuName": skuName,
		"groupPrice": act.GroupPrice, "groupCapacity": act.GroupCapacity,
		"startAt": act.StartAt, "endAt": act.EndAt,
		"maxGroups": act.MaxGroups, "openedCount": act.OpenedCount,
		"effectiveText": effectiveText(&act, time.Now()),
	}, nil
}

func (s *GroupBuyService) activityNames(ctx context.Context, a *model.GroupBuyActivity) (pkgTitle, skuName string) {
	s.db.WithContext(ctx).Model(&model.CoursePackage{}).Select("title").
		Where("id = ?", a.PackageID).Scan(&pkgTitle)
	s.db.WithContext(ctx).Model(&model.PackageSKU{}).Select("name").
		Where("id = ?", a.SkuID).Scan(&skuName)
	return pkgTitle, skuName
}

// effectiveText 活动当前状态：上下架 + 时间窗综合判定
func effectiveText(a *model.GroupBuyActivity, now time.Time) string {
	if a.Status != 1 {
		return "已下架"
	}
	if now.Before(a.StartAt) {
		return "未开始"
	}
	if !now.Before(a.EndAt) {
		return "已结束"
	}
	return "进行中"
}
