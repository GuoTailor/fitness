package service

import (
	"context"
	"time"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
)

// ShopService 商城查询（学员端展示）。
type ShopService struct {
	db    *gorm.DB
	owner int64
}

func NewShopService(db *gorm.DB, owner int64) *ShopService {
	return &ShopService{db: db, owner: owner}
}

// PackageListItem 列表项（含最低价与评分聚合）
type PackageListItem struct {
	model.CoursePackage
	MinPrice    int64   `json:"minPrice"`
	RatingAvg   float64 `json:"ratingAvg"`
	RatingCount int     `json:"ratingCount"`
}

// ListPackages 上架课包分页。category/tags/keyword 均可空。
func (s *ShopService) ListPackages(ctx context.Context, category, tag, keyword string, page, pageSize int) ([]PackageListItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	q := s.db.WithContext(ctx).Model(&model.CoursePackage{}).
		Where("owner_id = ? AND status = 1", s.owner)
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if tag != "" {
		q = q.Where("tags LIKE ?", "%"+tag+"%")
	}
	if keyword != "" {
		q = q.Where("title LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pkgs []model.CoursePackage
	if err := q.Order("sort DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&pkgs).Error; err != nil {
		return nil, 0, err
	}
	items, err := s.enrich(ctx, pkgs)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil // X1：返回真实总数，前端分页依赖 total
}

// enrich 批量补充最低价与评分聚合（X1：两条 GROUP BY 预取，替代每课包 2 条 N+1）
func (s *ShopService) enrich(ctx context.Context, pkgs []model.CoursePackage) ([]PackageListItem, error) {
	items := make([]PackageListItem, 0, len(pkgs))
	if len(pkgs) == 0 {
		return items, nil
	}
	ids := make([]int64, 0, len(pkgs))
	for _, p := range pkgs {
		ids = append(ids, p.ID)
	}
	// 最低在售规格价（批量）
	type minRow struct {
		PackageID int64
		MinPrice  *int64
	}
	var minRows []minRow
	if err := s.db.WithContext(ctx).Model(&model.PackageSKU{}).
		Select("package_id, MIN(price) AS min_price").
		Where("package_id IN ? AND status = 1", ids).
		Group("package_id").Scan(&minRows).Error; err != nil {
		return nil, err
	}
	minMap := make(map[int64]int64, len(minRows))
	for _, r := range minRows {
		if r.MinPrice != nil {
			minMap[r.PackageID] = *r.MinPrice
		}
	}
	// 评分聚合（批量，仅已通过审核的评价）
	type rateRow struct {
		PackageID int64
		Avg       float64
		Cnt       int
	}
	var rateRows []rateRow
	if err := s.db.WithContext(ctx).Model(&model.Review{}).
		Select("package_id, COALESCE(AVG(rating),0) AS avg, COUNT(*) AS cnt").
		Where("target_type = ? AND package_id IN ? AND status = ?",
			model.ReviewTargetPackage, ids, model.ReviewStatusApproved).
		Group("package_id").Scan(&rateRows).Error; err != nil {
		return nil, err
	}
	rateMap := make(map[int64]rateRow, len(rateRows))
	for _, r := range rateRows {
		rateMap[r.PackageID] = r
	}
	for _, p := range pkgs {
		it := PackageListItem{CoursePackage: p}
		if v, ok := minMap[p.ID]; ok {
			it.MinPrice = v
		}
		if r, ok := rateMap[p.ID]; ok {
			it.RatingAvg = r.Avg
			it.RatingCount = r.Cnt
		}
		items = append(items, it)
	}
	return items, nil
}

// PackageDetail 详情：规格 + 在售教练 + 评分 + 精选评价若干。
// userID>0 时附带新人价资格（server 下单时仍终判，此处仅供展示）。
func (s *ShopService) PackageDetail(ctx context.Context, id, userID int64) (map[string]interface{}, error) {
	var pkg model.CoursePackage
	err := s.db.WithContext(ctx).
		Where("id = ? AND owner_id = ? AND status = 1", id, s.owner).First(&pkg).Error
	if err != nil {
		return nil, errs.NotFound("课包不存在或已下架")
	}
	var skus []model.PackageSKU
	s.db.WithContext(ctx).Where("package_id = ? AND status = 1", id).Order("sort, price").Find(&skus)

	newcomerOK := false
	if userID > 0 {
		newcomerOK = newcomerEligible(ctx, s.db, userID)
	}
	type skuView struct {
		model.PackageSKU
		NewcomerEligible bool `json:"newcomerEligible"` // 设有新人价且当前用户可享
		PromoActive      bool `json:"promoActive"`      // 限时活动价当前生效
		// 拼团限时活动（P3）：生效时覆盖 SKU 常开拼团配置（上方 GroupPrice/GroupCapacity 已改写）
		GroupActivityId    int64      `json:"groupActivityId"`
		GroupActivityTitle string     `json:"groupActivityTitle"`
		GroupActivityEndAt *time.Time `json:"groupActivityEndAt"`
	}
	skuViews := make([]skuView, 0, len(skus))
	for _, sk := range skus {
		v := skuView{PackageSKU: sk}
		v.NewcomerEligible = newcomerOK && sk.NewcomerPrice != nil && *sk.NewcomerPrice > 0
		v.PromoActive = PromoActive(&sk, time.Now())
		if act := ActiveActivityFor(ctx, s.db, sk.ID); act != nil {
			v.GroupPrice = act.GroupPrice
			v.GroupCapacity = act.GroupCapacity
			v.GroupActivityId = act.ID
			v.GroupActivityTitle = act.Title
			endAt := act.EndAt
			v.GroupActivityEndAt = &endAt
		}
		skuViews = append(skuViews, v)
	}

	var coaches []model.Coach
	s.db.WithContext(ctx).
		Joins("JOIN coach_package cp ON cp.coach_id = coach.id").
		Where("cp.package_id = ? AND coach.status = 1", id).
		Find(&coaches)

	var ratingAvg float64
	var ratingCount int64
	s.db.WithContext(ctx).Model(&model.Review{}).
		Where("target_type = ? AND package_id = ? AND status = ?", model.ReviewTargetPackage, id, model.ReviewStatusApproved).
		Select("COALESCE(AVG(rating),0), COUNT(*)").Row().Scan(&ratingAvg, &ratingCount)

	var featured []model.Review
	s.db.WithContext(ctx).
		Where("target_type = ? AND package_id = ? AND status = ?", model.ReviewTargetPackage, id, model.ReviewStatusApproved).
		Order("featured DESC, rating ASC, id DESC").Limit(5).Find(&featured)

	return map[string]interface{}{
		"package":     pkg,
		"skus":        skuViews,
		"coaches":     coaches,
		"ratingAvg":   ratingAvg,
		"ratingCount": ratingCount,
		"reviews":     featured,
	}, nil
}

// ListCoaches 在职教练分页（可按专长关键词与最低评分筛选）
func (s *ShopService) ListCoaches(ctx context.Context, specialty, keyword string, minRating float64, page, pageSize int) ([]model.Coach, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	q := s.db.WithContext(ctx).Model(&model.Coach{}).Where("owner_id = ? AND status = 1", s.owner)
	if specialty != "" {
		q = q.Where("specialties LIKE ?", "%"+specialty+"%")
	}
	if keyword != "" {
		q = q.Where("name LIKE ?", "%"+keyword+"%")
	}
	if minRating > 0 {
		q = q.Where("rating_avg >= ?", minRating)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var coaches []model.Coach
	if err := q.Order("rating_avg DESC, sort DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&coaches).Error; err != nil {
		return nil, 0, err
	}
	return coaches, total, nil
}

// CoachDetail 教练主页：资料 + 在售课包 + 可约时段模板（M3 起提供实时 slots）
func (s *ShopService) CoachDetail(ctx context.Context, id int64) (map[string]interface{}, error) {
	var coach model.Coach
	err := s.db.WithContext(ctx).
		Where("id = ? AND owner_id = ? AND status = 1", id, s.owner).First(&coach).Error
	if err != nil {
		return nil, errs.NotFound("教练不存在或已停用")
	}
	var pkgs []model.CoursePackage
	if err := s.db.WithContext(ctx).
		Joins("JOIN coach_package cp ON cp.package_id = course_package.id").
		Where("cp.coach_id = ? AND course_package.status = 1", id).
		Order("course_package.sort DESC").Find(&pkgs).Error; err != nil {
		return nil, errs.Server(err, "查询教练课包失败")
	}
	// 复用批量聚合（X1：最低价 + 评分一次取回，与列表语义一致）
	items, err := s.enrich(ctx, pkgs)
	if err != nil {
		return nil, errs.Server(err, "查询课包聚合失败")
	}
	var schedules []model.CoachSchedule
	s.db.WithContext(ctx).Where("coach_id = ? AND status = 1", id).Order("weekday, start_minute").Find(&schedules)
	var reviews []model.Review
	s.db.WithContext(ctx).
		Where("target_type = ? AND coach_id = ? AND status = ?",
			model.ReviewTargetCoach, id, model.ReviewStatusApproved).
		Order("featured DESC, id DESC").Limit(5).Find(&reviews)
	return map[string]interface{}{
		"coach":     coach,
		"packages":  items,
		"schedules": schedules,
		"reviews":   reviews,
	}, nil
}

// Home 首页：Banner + 热销课包 + 高分教练
func (s *ShopService) Home(ctx context.Context) (map[string]interface{}, error) {
	var banners []model.Banner
	s.db.WithContext(ctx).Where("owner_id = ? AND status = 1", s.owner).Order("sort DESC, id DESC").Limit(5).Find(&banners)

	var hotPkgs []model.CoursePackage
	s.db.WithContext(ctx).Where("owner_id = ? AND status = 1", s.owner).
		Order("sold_count DESC, id DESC").Limit(6).Find(&hotPkgs)
	hotItems, _ := s.enrich(ctx, hotPkgs) // 首页降级：聚合失败返回空最低价

	var coaches []model.Coach
	s.db.WithContext(ctx).Where("owner_id = ? AND status = 1", s.owner).
		Order("rating_avg DESC, id DESC").Limit(4).Find(&coaches)

	return map[string]interface{}{"banners": banners, "hotPackages": hotItems, "coaches": coaches}, nil
}
