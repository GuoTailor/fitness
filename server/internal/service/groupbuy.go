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

// GroupBuyService 拼团：SKU 配拼团价+成团人数，任意用户开团/参团，
// 各自按拼团价支付并入账；到期未成团由定时任务自动原路退款（复用退款审批链路）。
// 拼团价不与券/新人价/活动价叠加（独立建单，不走三方比价）。
type GroupBuyService struct {
	db     *gorm.DB
	cfg    *ConfigService
	notify *NotifyService // 可为 nil
}

func NewGroupBuyService(db *gorm.DB, cfg *ConfigService) *GroupBuyService {
	return &GroupBuyService{db: db, cfg: cfg}
}

func (s *GroupBuyService) WithNotify(n *NotifyService) *GroupBuyService {
	s.notify = n
	return s
}

// Open 开团：限时活动生效期内按活动配置开团（名额原子扣减），
// 否则回落 SKU 常开拼团配置；建团 + 拼团价订单（返回订单走 prepay；拼团单与成员行同事务）
func (s *GroupBuyService) Open(ctx context.Context, userID, skuID int64) (*model.Order, error) {
	sku, pkg, err := s.loadSku(ctx, skuID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	act := ActiveActivityFor(ctx, s.db, skuID)
	if act == nil && (sku.GroupPrice <= 0 || sku.GroupCapacity < 2) {
		return nil, errs.Biz("该课包未开启拼团")
	}
	if act == nil && sku.GroupPrice >= sku.Price {
		return nil, errs.Biz("拼团价配置异常")
	}
	price, capacity, actID := sku.GroupPrice, sku.GroupCapacity, int64(0)
	hours := 0
	if act != nil {
		price, capacity, actID = act.GroupPrice, act.GroupCapacity, act.ID
		hours = act.ExpireHours
	}
	if hours <= 0 {
		hours = s.cfg.GetInt(ctx, model.CfgGroupExpireHours, 48)
		if hours <= 0 {
			hours = 48
		}
	}
	var order model.Order
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if act != nil {
			if err := claimActivity(tx, act, now); err != nil {
				return err
			}
		}
		gb := model.GroupBuy{
			OwnerID: pkg.OwnerID, SkuID: sku.ID, PackageID: sku.PackageID,
			Capacity: capacity, JoinedCount: 1, // 团长开团即占一席（P0-1）
			Price:  price,
			Status: model.GroupBuyEnrolling, LeaderUserID: userID,
			ActivityID: actID,
			ExpireAt:   now.Add(time.Duration(hours) * time.Hour),
		}
		if err := tx.Create(&gb).Error; err != nil {
			return err
		}
		order, err = createGroupMemberOrder(tx, &gb, userID, sku, pkg.Title)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// Join 参团：团未满未过期时可按拼团价下单。
// 占席用「状态/有效期/容量」同一条件 UPDATE 原子完成（P0-1），并发不超卖；
// 占席后建单失败（含重复参团）整体回滚，座位自动释放。
func (s *GroupBuyService) Join(ctx context.Context, userID, gbID int64) (*model.Order, error) {
	var gb model.GroupBuy
	if err := s.db.WithContext(ctx).First(&gb, gbID).Error; err != nil {
		return nil, errs.NotFound("拼团不存在")
	}
	sku, pkg, err := s.loadSku(ctx, gb.SkuID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var order model.Order
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.GroupBuy{}).
			Where("id = ? AND status = ? AND expire_at > ? AND joined_count < capacity",
				gbID, model.GroupBuyEnrolling, now).
			UpdateColumn("joined_count", gorm.Expr("joined_count + 1"))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			var cur model.GroupBuy
			if err := tx.First(&cur, gbID).Error; err != nil {
				return errs.NotFound("拼团不存在")
			}
			switch {
			case cur.Status != model.GroupBuyEnrolling:
				return errs.Biz("该团已结束")
			case !now.Before(cur.ExpireAt):
				return errs.Biz("该团已过期")
			default:
				return errs.Biz("该团已满员")
			}
		}
		order, err = createGroupMemberOrder(tx, &gb, userID, sku, pkg.Title)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// createGroupMemberOrder 事务内建拼团订单（快照入账字段）+ 成员行（uk 防重复参团）
func createGroupMemberOrder(tx *gorm.DB, gb *model.GroupBuy, userID int64, sku *model.PackageSKU, pkgTitle string) (model.Order, error) {
	var exist model.GroupBuyMember
	if err := tx.Where("group_buy_id = ? AND user_id = ?", gb.ID, userID).First(&exist).Error; err == nil {
		return model.Order{}, errs.Biz("你已参与该团")
	}
	unitPrice := int64(0)
	if sku.Sessions > 0 {
		unitPrice = gb.Price / int64(sku.Sessions)
	}
	order := model.Order{
		OrderNo: genOrderNo(), OwnerID: gb.OwnerID, UserID: userID,
		PackageID: gb.PackageID, SkuID: gb.SkuID,
		Title:  fmt.Sprintf("%s·%s", pkgTitle, sku.Name),
		Amount: gb.Price, Status: model.OrderStatusPending,
		SkuSessions: sku.Sessions, SkuValidDays: sku.ValidDays, SkuUnitPrice: unitPrice,
		GroupBuyID: gb.ID,
	}
	if err := tx.Create(&order).Error; err != nil {
		return model.Order{}, err
	}
	if err := tx.Create(&model.Payment{OrderID: order.ID, Amount: order.Amount, Status: "created"}).Error; err != nil {
		return model.Order{}, err
	}
	member := model.GroupBuyMember{GroupBuyID: gb.ID, UserID: userID, OrderID: order.ID}
	if err := tx.Create(&member).Error; err != nil {
		if isDuplicateKey(err) {
			return model.Order{}, errs.Biz("你已参与该团")
		}
		return model.Order{}, err
	}
	return order, nil
}

// OnOrderPaid 支付成功联动（HandlePaySuccess 事务外调用）：拼团单尝试成团
func (s *GroupBuyService) OnOrderPaid(ctx context.Context, orderID int64) {
	var order model.Order
	if err := s.db.WithContext(ctx).Select("id, group_buy_id").
		First(&order, orderID).Error; err != nil || order.GroupBuyID == 0 {
		return
	}
	s.TrySucceed(ctx, order.GroupBuyID)
}

// TrySucceed 已支付人数达标 → 条件 UPDATE enrolling→success（幂等闸门）→ 通知全员。
// 满员却被置 failed 属联动丢失，打告警由人工/任务兜底（P0-1）。
func (s *GroupBuyService) TrySucceed(ctx context.Context, gbID int64) {
	var gb model.GroupBuy
	if err := s.db.WithContext(ctx).First(&gb, gbID).Error; err != nil {
		return
	}
	paid, err := s.paidCount(ctx, gbID)
	if err != nil || paid < int64(gb.Capacity) {
		return
	}
	now := time.Now()
	res := s.db.WithContext(ctx).Model(&model.GroupBuy{}).
		Where("id = ? AND status = ?", gbID, model.GroupBuyEnrolling).
		Updates(map[string]interface{}{"status": model.GroupBuySuccess, "success_at": now})
	if res.Error != nil {
		log.Printf("[groupbuy][ALERT] 成团状态落库失败 gb=%d: %v", gbID, res.Error)
		return
	}
	if res.RowsAffected == 0 {
		var cur model.GroupBuy
		if err := s.db.WithContext(ctx).Select("status").First(&cur, gbID).Error; err == nil &&
			cur.Status == model.GroupBuyFailed {
			log.Printf("[groupbuy][ALERT] 满员团处于失败态（联动丢失）gb=%d paid=%d capacity=%d",
				gbID, paid, gb.Capacity)
		}
		return // 已成团幂等
	}
	// 通知全部已支付成员（biz=团 ID，uk 天然去重）
	var members []model.GroupBuyMember
	s.db.WithContext(ctx).Where("group_buy_id = ?", gbID).Find(&members)
	var title string
	s.db.WithContext(ctx).Model(&model.CoursePackage{}).Select("title").
		Where("id = ?", gb.PackageID).Scan(&title)
	for _, m := range members {
		tryNotify(ctx, s.notify, m.UserID, model.NotifySceneGroupBuySuccess, gbID,
			map[string]string{"thing1": "拼团成功，课包已到账", "date2": now.Format("2006-01-02")})
	}
}

// ListOpen 某 SKU 的进行中团（进度 + 团长昵称）
func (s *GroupBuyService) ListOpen(ctx context.Context, skuID int64) ([]map[string]interface{}, error) {
	var gbs []model.GroupBuy
	if err := s.db.WithContext(ctx).
		Where("sku_id = ? AND status = ? AND expire_at > ?", skuID, model.GroupBuyEnrolling, time.Now()).
		Order("id DESC").Limit(20).Find(&gbs).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(gbs))
	for _, gb := range gbs {
		paid, _ := s.paidCount(ctx, gb.ID)
		var leader model.User
		name := "团员"
		if err := s.db.WithContext(ctx).Select("nickname").First(&leader, gb.LeaderUserID).Error; err == nil && leader.Nickname != "" {
			name = leader.Nickname
		}
		out = append(out, map[string]interface{}{
			"id": gb.ID, "capacity": gb.Capacity, "paidCount": paid,
			"price": gb.Price, "expireAt": gb.ExpireAt, "leaderName": name,
		})
	}
	return out, nil
}

// MyGroups 我参与的团（含订单状态）
func (s *GroupBuyService) MyGroups(ctx context.Context, userID int64) ([]map[string]interface{}, error) {
	var rows []struct {
		model.GroupBuyMember
		GBStatus  int8       `gorm:"column:gb_status"`
		Capacity  int        `gorm:"column:capacity"`
		Price     int64      `gorm:"column:price"`
		ExpireAt  time.Time  `gorm:"column:expire_at"`
		SuccessAt *time.Time `gorm:"column:success_at"`
		Title     string     `gorm:"column:title"`
		PkgID     int64      `gorm:"column:package_id"`
	}
	err := s.db.WithContext(ctx).
		Table("group_buy_member AS m").
		Select("m.*, g.status AS gb_status, g.capacity, g.price, g.expire_at, g.success_at, g.package_id, p.title").
		Joins("JOIN group_buy g ON g.id = m.group_buy_id").
		Joins("JOIN course_package p ON p.id = g.package_id").
		Where("m.user_id = ?", userID).
		Order("m.id DESC").Limit(20).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		statusText := "报名中"
		switch r.GBStatus {
		case model.GroupBuySuccess:
			statusText = "已成团"
		case model.GroupBuyFailed:
			statusText = "未成团（已自动退款）"
		}
		out = append(out, map[string]interface{}{
			"groupBuyId": r.GroupBuyID, "capacity": r.Capacity, "price": r.Price,
			"status": r.GBStatus, "statusText": statusText, "expireAt": r.ExpireAt,
			"orderId": r.OrderID, "title": r.Title, "packageId": r.PkgID,
		})
	}
	return out, nil
}

// Detail 团详情（分享落地页）：进度 + 我的订单状态
func (s *GroupBuyService) Detail(ctx context.Context, userID, gbID int64) (map[string]interface{}, error) {
	var gb model.GroupBuy
	if err := s.db.WithContext(ctx).First(&gb, gbID).Error; err != nil {
		return nil, errs.NotFound("拼团不存在")
	}
	paid, _ := s.paidCount(ctx, gbID)
	pkgTitle := ""
	s.db.WithContext(ctx).Model(&model.CoursePackage{}).Select("title").
		Where("id = ?", gb.PackageID).Scan(&pkgTitle)
	leaderName := "团员"
	var leader model.User
	if err := s.db.WithContext(ctx).Select("nickname").First(&leader, gb.LeaderUserID).Error; err == nil && leader.Nickname != "" {
		leaderName = leader.Nickname
	}
	out := map[string]interface{}{
		"id": gb.ID, "skuId": gb.SkuID, "packageId": gb.PackageID, "title": pkgTitle,
		"capacity": gb.Capacity, "price": gb.Price, "paidCount": paid,
		"status": gb.Status, "expireAt": gb.ExpireAt, "leaderName": leaderName,
		"joined": false, "myOrderPaid": false,
	}
	if gb.ActivityID > 0 {
		var act model.GroupBuyActivity
		if s.db.WithContext(ctx).Select("title").First(&act, gb.ActivityID).Error == nil {
			out["activityTitle"] = act.Title
		}
	}
	// 我在该团的订单（最新一单）
	var order model.Order
	if err := s.db.WithContext(ctx).
		Where("group_buy_id = ? AND user_id = ?", gbID, userID).
		Order("id DESC").First(&order).Error; err == nil {
		out["joined"] = true
		out["myOrderId"] = order.ID
		out["myOrderNo"] = order.OrderNo
		out["myOrderStatus"] = order.Status
		out["myOrderPaid"] = order.Status == model.OrderStatusPaid ||
			order.Status == model.OrderStatusRefund || order.Status == model.OrderStatusRefunded
	}
	return out, nil
}

// AdminList 后台拼团列表
func (s *GroupBuyService) AdminList(ctx context.Context, status int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.GroupBuy{})
	if status > 0 {
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
	var gbs []model.GroupBuy
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&gbs).Error; err != nil {
		return nil, 0, err
	}
	out := make([]map[string]interface{}, 0, len(gbs))
	for _, gb := range gbs {
		paid, _ := s.paidCount(ctx, gb.ID)
		statusText := map[int8]string{
			model.GroupBuyEnrolling: "报名中", model.GroupBuySuccess: "已成团", model.GroupBuyFailed: "未成团",
		}[gb.Status]
		activityTitle := ""
		if gb.ActivityID > 0 {
			s.db.WithContext(ctx).Model(&model.GroupBuyActivity{}).Select("title").
				Where("id = ?", gb.ActivityID).Scan(&activityTitle)
		}
		out = append(out, map[string]interface{}{
			"id": gb.ID, "skuId": gb.SkuID, "packageId": gb.PackageID, "capacity": gb.Capacity,
			"price": gb.Price, "status": gb.Status, "statusText": statusText,
			"paidCount": paid, "leaderUserId": gb.LeaderUserID,
			"activityId": gb.ActivityID, "activityTitle": activityTitle,
			"expireAt": gb.ExpireAt, "successAt": gb.SuccessAt, "createdAt": gb.CreatedAt,
		})
	}
	return out, total, nil
}

// paidCount 已支付成员数（仅 paid：退款终态不再占有效名额，
// 否则占席释放后新成员加入会超员成团，P0-1）
func (s *GroupBuyService) paidCount(ctx context.Context, gbID int64) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&model.Order{}).
		Where("group_buy_id = ? AND status = ?", gbID, model.OrderStatusPaid).
		Count(&n).Error
	return n, err
}

func (s *GroupBuyService) loadSku(ctx context.Context, skuID int64) (*model.PackageSKU, *model.CoursePackage, error) {
	var sku model.PackageSKU
	if err := s.db.WithContext(ctx).Where("id = ? AND status = 1", skuID).First(&sku).Error; err != nil {
		return nil, nil, errs.NotFound("课包规格不存在或已停售")
	}
	var pkg model.CoursePackage
	if err := s.db.WithContext(ctx).Where("id = ? AND status = 1", sku.PackageID).First(&pkg).Error; err != nil {
		return nil, nil, errs.NotFound("课包不存在或已下架")
	}
	return &sku, &pkg, nil
}
