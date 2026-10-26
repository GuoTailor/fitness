package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"fitness/internal/errs"
	"fitness/internal/model"
	"fitness/internal/wechat"
)

// refundAbnormalLimit 退款连续异常次数阈值：达到后置 failed 转人工（P1-4）
const refundAbnormalLimit = 3

// OrderService 订单状态机：pending → paid →（refunding → refunded / closed）。
// 支付成功回调（真实或 mock）统一走 HandlePaySuccess，全程幂等。
type OrderService struct {
	db       *gorm.DB
	cfg      *ConfigService
	pay      wechat.PaymentProvider
	referral *ReferralService // 可为 nil：支付成功后的老带新奖励联动
	groupBuy *GroupBuyService // 可为 nil：支付成功后的拼团成团检查
}

func NewOrderService(db *gorm.DB, cfg *ConfigService, pay wechat.PaymentProvider) *OrderService {
	return &OrderService{db: db, cfg: cfg, pay: pay}
}

// WithReferralReward 接入老带新奖励（支付成功后检查邀请关系）
func (s *OrderService) WithReferralReward(r *ReferralService) *OrderService {
	s.referral = r
	return s
}

// WithGroupBuy 接入拼团服务（支付成功后检查成团）
func (s *OrderService) WithGroupBuy(g *GroupBuyService) *OrderService {
	s.groupBuy = g
	return s
}

func genOrderNo() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return time.Now().Format("20060102150405") + hex.EncodeToString(b)
}

func genRefundNo() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "RF" + time.Now().Format("20060102150405") + hex.EncodeToString(b)
}

// Create 下单（P1 起支持优惠券与新人价）。
// 下单即快照 SKU 的课时/有效期/单价（M5）：后台删改规格不影响本单入账与退款。
// 金额规则：券优惠与新人价不叠加、只取其一（取低价，规划 §7.1）；
// 优惠一律封顶 price-1 分（微信要求实付>0）；券核销（usable→used 绑定订单）
// 在订单事务内完成，任何不满足整体回滚。
func (s *OrderService) Create(ctx context.Context, userID int64, skuID int64, couponID int64) (*model.Order, error) {
	var sku model.PackageSKU
	if err := s.db.WithContext(ctx).Where("id = ? AND status = 1", skuID).First(&sku).Error; err != nil {
		return nil, errs.NotFound("课包规格不存在或已停售")
	}
	if sku.Price <= 0 || sku.Sessions <= 0 || sku.ValidDays <= 0 {
		return nil, errs.BadParam("课包规格数据异常，请联系店家") // M6 兜底：脏 SKU 不可下单
	}
	var pkg model.CoursePackage
	if err := s.db.WithContext(ctx).Where("id = ? AND status = 1", sku.PackageID).First(&pkg).Error; err != nil {
		return nil, errs.NotFound("课包不存在或已下架")
	}
	unitPrice := int64(0)
	if sku.Sessions > 0 {
		unitPrice = sku.Price / int64(sku.Sessions)
	}

	// ---- 优惠计算（事务外预读，事务内终校验）----
	newcomerOff := newcomerDiscountOf(ctx, s.db, userID, &sku)
	promoOff := promoDiscountOf(&sku, time.Now())
	var couponOff int64
	isGiftCoupon := false
	giftAmount := 0 // 赠课券赠送课时数（预读快照，事务内核销时兜底校正）
	if couponID != 0 {
		var uc model.UserCoupon
		if err := s.db.WithContext(ctx).
			Where("id = ? AND user_id = ? AND status = ? AND expire_at > ?",
				couponID, userID, model.CouponUsable, time.Now()).First(&uc).Error; err != nil {
			return nil, errs.Biz("优惠券不可用或已过期")
		}
		var tpl model.CouponTemplate
		if err := s.db.WithContext(ctx).First(&tpl, uc.TemplateID).Error; err != nil {
			return nil, errs.Biz("优惠券信息异常")
		}
		if tpl.PackageID != 0 && tpl.PackageID != pkg.ID {
			return nil, errs.Biz("该优惠券不适用于此课包")
		}
		if tpl.Type == model.CouponTypeGift {
			isGiftCoupon = true // 赠课券不降价：绑单赠课时，可与价格类优惠叠加
			giftAmount = int(tpl.Amount)
		} else {
			couponOff = CouponBenefit(&tpl, sku.Price)
			if couponOff <= 0 {
				return nil, errs.Biz("订单金额未满足用券条件")
			}
		}
	}
	// 三方比价（规划 §7.1 不叠加、只应用单一最大优惠）。
	// 同额优先级 promo > newcomer > coupon：活动价不限次，同额时保住
	// 新人一次性资格与券（不核销、不消耗）。
	useNewcomer, usePromo := false, false
	applyCoupon := int64(0)
	if isGiftCoupon {
		applyCoupon = couponID // 赠课券独立绑单，不参与比价
	}
	amount := sku.Price
	best := promoOff
	if newcomerOff > best {
		best = newcomerOff
	}
	if couponOff > best {
		best = couponOff
	}
	switch {
	case best <= 0:
		// 无价格优惠
	case couponOff == best && couponOff > promoOff && couponOff > newcomerOff:
		applyCoupon = couponID
		amount = sku.Price - couponOff
	case newcomerOff == best && newcomerOff >= couponOff && newcomerOff > promoOff:
		useNewcomer = true
		amount = sku.Price - newcomerOff
	default:
		usePromo = true
		amount = sku.Price - promoOff
	}
	if amount < 1 {
		amount = 1
	}

	newcomerUsed := int8(0)
	if useNewcomer {
		newcomerUsed = 1
	}
	promoUsed := int8(0)
	if usePromo {
		promoUsed = 1
	}
	couponAmount := int64(0)
	if applyCoupon != 0 && !isGiftCoupon {
		couponAmount = couponOff
	}
	order := model.Order{
		OrderNo:      genOrderNo(),
		OwnerID:      pkg.OwnerID,
		UserID:       userID,
		PackageID:    pkg.ID,
		SkuID:        sku.ID,
		Title:        fmt.Sprintf("%s·%s", pkg.Title, sku.Name),
		Amount:       amount,
		CouponID:     applyCoupon,
		CouponAmount: couponAmount,
		Status:       model.OrderStatusPending,
		SkuSessions:  sku.Sessions,
		SkuValidDays: sku.ValidDays,
		SkuUnitPrice: unitPrice,
		NewcomerUsed: newcomerUsed,
		PromoUsed:    promoUsed,
		GiftSessions: giftAmount,
	}
	// 订单与支付单同事务创建（M11）：避免只写一半；券核销同事务，失败整体回滚
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Payment{OrderID: order.ID, Amount: order.Amount, Status: "created"}).Error; err != nil {
			return err
		}
		if applyCoupon != 0 {
			// 事务内终校验并核销；模板极小概率在预读后被改动时以核销时的结果修正订单
			benefit, gift, cerr := ApplyCouponForOrder(tx, userID, applyCoupon, order.ID, pkg.ID, sku.Price)
			if cerr != nil {
				return cerr
			}
			if !isGiftCoupon && benefit != couponAmount {
				fixed := sku.Price - benefit
				if fixed < 1 {
					fixed = 1
				}
				if err := tx.Model(&model.Order{}).Where("id = ?", order.ID).
					Updates(map[string]interface{}{"amount": fixed, "coupon_amount": benefit}).Error; err != nil {
					return err
				}
				if err := tx.Model(&model.Payment{}).Where("order_id = ?", order.ID).
					Update("amount", fixed).Error; err != nil {
					return err
				}
				order.Amount, order.CouponAmount = fixed, benefit
			}
			if isGiftCoupon && gift != order.GiftSessions {
				if err := tx.Model(&model.Order{}).Where("id = ?", order.ID).
					Update("gift_sessions", gift).Error; err != nil {
					return err
				}
				order.GiftSessions = gift
			}
		}
		return nil
	})
	if err != nil {
		if ae, ok := err.(*errs.AppError); ok {
			return nil, ae
		}
		return nil, errs.Server(err, "创建订单失败")
	}
	return &order, nil
}

// newcomerDiscountOf 新人价可享优惠额（0=不可享）：SKU 设有新人价 + 已绑手机号 +
// 该手机号从未用过新人价（status<>closed 的新人价订单判重，唯一索引兜底一号一人）。
func newcomerDiscountOf(ctx context.Context, db *gorm.DB, userID int64, sku *model.PackageSKU) int64 {
	if sku.NewcomerPrice == nil || *sku.NewcomerPrice <= 0 || *sku.NewcomerPrice >= sku.Price {
		return 0
	}
	if !newcomerEligible(ctx, db, userID) {
		return 0
	}
	return sku.Price - *sku.NewcomerPrice
}

// promoDiscountOf 限时活动价可享优惠额（0=不可享）：SKU 三列齐设且 now 落在 [start, end) 内。
func promoDiscountOf(sku *model.PackageSKU, now time.Time) int64 {
	if sku.PromoPrice == nil || sku.PromoStart == nil || sku.PromoEnd == nil {
		return 0
	}
	p := *sku.PromoPrice
	if p <= 0 || p >= sku.Price || now.Before(*sku.PromoStart) || !now.Before(*sku.PromoEnd) {
		return 0
	}
	return sku.Price - p
}

// PromoActive 详情页展示用：活动价当前是否生效
func PromoActive(sku *model.PackageSKU, now time.Time) bool {
	return promoDiscountOf(sku, now) > 0
}

// newcomerEligible 手机号维度的新人资格（详情页展示与下单共用一个判定）
func newcomerEligible(ctx context.Context, db *gorm.DB, userID int64) bool {
	var user model.User
	if err := db.WithContext(ctx).Select("id", "phone").First(&user, userID).Error; err != nil || user.Phone == nil {
		return false
	}
	phone := *user.Phone
	if phone == "" {
		return false
	}
	var used int64
	if err := db.WithContext(ctx).Model(&model.Order{}).
		Joins("JOIN user u ON u.id = orders.user_id").
		Where("u.phone = ? AND orders.newcomer_used = 1 AND orders.status <> ?", phone, model.OrderStatusClosed).
		Limit(1).Count(&used).Error; err != nil {
		return false
	}
	return used == 0
}

// Prepay 发起支付。openid 一律取登录用户的服务端记录（E2：不信任客户端传入）。
// channel：jsapi（默认，小程序拉起支付）/ native（网页端扫码，返回 code_url）。
// mock 模式：直接把该单置为已支付（走 HandlePaySuccess 幂等链路）。
func (s *OrderService) Prepay(ctx context.Context, orderID, userID int64, channel string) (*wechat.PrepayResult, error) {
	order, err := s.getOwnedOrder(ctx, orderID, userID)
	if err != nil {
		return nil, err
	}
	if order.Status != model.OrderStatusPending {
		return nil, errs.Biz("订单当前状态不可支付")
	}
	switch channel {
	case "", "jsapi", "native":
	default:
		return nil, errs.BadParam("不支持的支付通道")
	}
	var res *wechat.PrepayResult
	if channel == "native" {
		res, err = s.pay.PrepayNative(ctx, order.OrderNo, order.Amount, order.Title)
	} else {
		var user model.User
		if err := s.db.WithContext(ctx).Select("id", "openid").First(&user, userID).Error; err != nil {
			return nil, errs.NotFound("用户不存在")
		}
		res, err = s.pay.Prepay(ctx, order.OrderNo, user.OpenID, order.Amount, order.Title)
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeBiz, "发起支付失败", err)
	}
	if res.MockPaid {
		// 模拟微信回调：直接进入支付成功链路（幂等，重复调用安全）
		if err := s.HandlePaySuccess(ctx, order.OrderNo, "mocktxn-"+order.OrderNo, "mock", order.Amount); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// MyOrder 订单详情（本人校验，他人 404）——网页端扫码支付轮询用
func (s *OrderService) MyOrder(ctx context.Context, orderID, userID int64) (*model.Order, error) {
	return s.getOwnedOrder(ctx, orderID, userID)
}

// MyOrders 我的订单（倒序分页；statuses 非空时按状态过滤，逗号分隔白名单校验）
var orderStatusWhitelist = map[string]bool{
	model.OrderStatusPending: true, model.OrderStatusPaid: true, model.OrderStatusClosed: true,
	model.OrderStatusRefund: true, model.OrderStatusRefunded: true,
}

func (s *OrderService) MyOrders(ctx context.Context, userID int64, page, pageSize int, statuses string) ([]model.Order, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	q := s.db.WithContext(ctx).Model(&model.Order{}).Where("user_id = ?", userID)
	if statuses != "" {
		var allowed []string
		for _, st := range strings.Split(statuses, ",") {
			st = strings.TrimSpace(st)
			if st == "" {
				continue
			}
			if !orderStatusWhitelist[st] {
				return nil, 0, errs.BadParam("不支持的订单状态: " + st)
			}
			allowed = append(allowed, st)
		}
		if len(allowed) > 0 {
			q = q.Where("status IN ?", allowed)
		}
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var orders []model.Order
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// ApplyRefund 学员申请退款：仅已支付订单；金额由后台审核时按规则计算。
// 闸门与建退款单同事务（E9）：建单失败订单不会卡在 refunding。
func (s *OrderService) ApplyRefund(ctx context.Context, orderID, userID int64, reason string) (*model.Refund, error) {
	order, err := s.getOwnedOrder(ctx, orderID, userID)
	if err != nil {
		return nil, err
	}
	// 已有处理中的退款单直接复用（重复提交幂等）
	var exist model.Refund
	if err := s.db.WithContext(ctx).Where("order_id = ? AND status = ?", orderID, model.RefundStatusPending).
		First(&exist).Error; err == nil {
		return &exist, nil
	}
	if order.Status != model.OrderStatusPaid {
		return nil, errs.Biz("当前订单状态不可申请退款")
	}
	refund := model.Refund{
		RefundNo: genRefundNo(), OrderID: order.ID, UserID: userID,
		Reason: reason, Status: model.RefundStatusPending,
	}
	// 状态闸门 paid → refunding（并发防重）+ 建退款单，同一事务
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Order{}).
			Where("id = ? AND status = ?", orderID, model.OrderStatusPaid).
			Update("status", model.OrderStatusRefund)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errs.Conflict("重复提交，请刷新查看")
		}
		if err := tx.Create(&refund).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errs.Server(err, "创建退款单失败")
	}
	return &refund, nil
}

// CreateAutoRefund 系统自动退款（拼团未成团）：与用户申请同一闸门链路
// （订单 paid→refunding + 建退款单同事务），金额由 ApproveRefund 重算落库。
// 幂等按上一张退款单的状态判定：pending/approved 复用；rejected 重新发起
// （被拒时订单已回 paid，闸门可正常通过）；failed/refunded 不再自动建单，
// 返回错误由调用方告警转人工——避免「被拒过的退款单」永久堵死自动退款。
func (s *OrderService) CreateAutoRefund(ctx context.Context, orderID, userID int64, reason string) (*model.Refund, error) {
	refund := model.Refund{
		RefundNo: genRefundNo(), OrderID: orderID, UserID: userID,
		Reason: reason, Status: model.RefundStatusPending,
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var last model.Refund
		err := tx.Where("order_id = ?", orderID).Order("id DESC").First(&last).Error
		switch {
		case err == nil && (last.Status == model.RefundStatusPending || last.Status == model.RefundStatusApproved):
			refund = last // 已在退款链路中：复用，不重复建单
			return nil
		case err == nil && last.Status == model.RefundStatusFailed:
			return errs.Conflict("该订单退款已转人工处理，跳过自动退款")
		case err == nil && last.Status == model.RefundStatusRefunded:
			return errs.Conflict("该订单已退款完成，跳过自动退款")
		case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		}
		// 无退款单或上一单被拒：重新发起
		res := tx.Model(&model.Order{}).
			Where("id = ? AND status = ?", orderID, model.OrderStatusPaid).
			Update("status", model.OrderStatusRefund)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errs.Conflict("订单当前状态不可退款")
		}
		return tx.Create(&refund).Error
	})
	if err != nil {
		return nil, err
	}
	return &refund, nil
}

// refundCalc 退款金额计算结果（审批时快照一并落库，P0-3）
type refundCalc struct {
	amount    int64
	note      string
	calcType  string // full / remainder / full_no_mp
	unitPrice int64  // 折算单价快照（全额退时记录订单快照价，仅存证）
}

// calcRefundAmount 退款金额规则：
// 无课时账户 → 全额；未开卡且支付未超过 refund.full_days 天 → 全额；
// 否则按「可退剩余课时 / 可退总课时」的比例折算实付金额（P0-2：
// 不能用 mp.UnitPrice=售价/总课时——整除向下取整，price<sessions 时单价为 0，无脏数据也会退 0）。
// 以调用时的最新余额为准（事务内重读）。
func (s *OrderService) calcRefundAmount(ctx context.Context, order *model.Order, mp *model.MemberPackage) (refundCalc, error) {
	fullDays := s.cfg.GetInt(ctx, model.CfgRefundFullDays, 7)
	if mp == nil {
		return refundCalc{amount: order.Amount, note: "无课时账户，全额退",
			calcType: "full_no_mp", unitPrice: order.SkuUnitPrice}, nil
	}
	if mp.Status == model.MPStatusInactive && order.PayTime != nil &&
		time.Since(*order.PayTime) <= time.Duration(fullDays)*24*time.Hour {
		return refundCalc{amount: order.Amount, note: fmt.Sprintf("未开卡且在 %d 天内，全额退", fullDays),
			calcType: "full", unitPrice: order.SkuUnitPrice}, nil
	}
	// 赠课不参与退款折算：可退总课时 = 总课时 - 赠课；剩余可退 = 可退总课时 - 已消耗
	payableTotal := mp.TotalSessions - mp.GiftSessions
	if payableTotal <= 0 {
		return refundCalc{note: "课时均为赠课，无可退金额"}, errs.Biz("课时均为赠课，无可退金额")
	}
	remaining := payableTotal - mp.UsedSessions
	if remaining <= 0 {
		return refundCalc{note: "剩余课时均为赠课或已用完，无可退金额"}, errs.Biz("课时已用完，无可退金额")
	}
	base := order.Amount - order.RefundAmount
	if base <= 0 {
		return refundCalc{note: "订单已无可退金额"}, errs.Biz("订单已无可退金额")
	}
	// 按实付金额比例折算并向上取整（防整除造 0，P0-2）
	amount := (base*int64(remaining) + int64(payableTotal) - 1) / int64(payableTotal)
	if amount > base {
		amount = base // 折算价不得高于实付余额
	}
	if amount <= 0 {
		return refundCalc{note: "折算金额为 0，无可退金额"}, errs.Biz("折算金额为 0，无可退金额")
	}
	unitPrice := (order.Amount + int64(payableTotal) - 1) / int64(payableTotal) // 存证快照（ceil）
	note := fmt.Sprintf("剩余可退 %d/%d 课时，按实付 %d 分折算退 %d 分",
		remaining, payableTotal, base, amount)
	if mp.GiftSessions > 0 {
		note += fmt.Sprintf("（%d 节赠课不参与退款）", mp.GiftSessions)
	}
	return refundCalc{
		amount:    amount,
		note:      note,
		calcType:  "remainder",
		unitPrice: unitPrice,
	}, nil
}

// ApproveRefund 后台同意退款。
// 顺序（P0-2/P0-3）：先抢 pending→approved 闸门（与拒绝互斥，杜绝"先打款后被拒"）→
// 事务内重读订单/课包、重算金额并落库（amount + 审批快照）+ 回收课时（条件更新绑定
// 读取时的余额，防打款窗口期消耗导致超退）→ 提交后再发起微信退款（out_refund_no
// 幂等，失败不回滚本地，由轮询任务重试）→ 查询推进终态（mock 恒 SUCCESS 即时 refunded）。
func (s *OrderService) ApproveRefund(ctx context.Context, refundID, adminID int64) (*model.Refund, error) {
	var refund model.Refund
	if err := s.db.WithContext(ctx).First(&refund, refundID).Error; err != nil {
		return nil, errs.NotFound("退款单不存在")
	}

	// 1) 事务：抢闸门（pending→approved）+ 重读重算 + 快照落库 + 回收课时。
	// 闸门与结算同事务（P1-2）：任一步失败整体回滚，退款单回 pending、订单仍 refunding，可重试；
	// 不再产生「approved 且 amount=0」的永久孤儿。
	var order model.Order
	var calc refundCalc
	var calcErr error
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Refund{}).
			Where("id = ? AND status = ?", refundID, model.RefundStatusPending).
			Updates(map[string]interface{}{"status": model.RefundStatusApproved, "admin_id": adminID})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errs.Conflict("该退款单已被处理")
		}
		if err := tx.First(&order, refund.OrderID).Error; err != nil {
			return errs.NotFound("订单不存在")
		}
		var m model.MemberPackage
		var mp *model.MemberPackage
		if err := tx.Where("order_id = ?", order.ID).First(&m).Error; err == nil {
			mp = &m
		} else if err != gorm.ErrRecordNotFound {
			return errs.Server(err, "查询课时账户失败") // E10：非 NotFound 不再吞成"全额退"
		}

		calc, calcErr = s.calcRefundAmount(ctx, &order, mp)
		if calcErr != nil {
			// 无可退金额：补偿写（退款单拒绝、订单回 paid）需提交生效，
			// 事务内 return err 会整体回滚，故置标记后在事务外返回错误
			tx.Model(&model.Refund{}).Where("id = ?", refundID).
				Updates(map[string]interface{}{"status": model.RefundStatusRejected, "reject_reason": calc.note})
			tx.Model(&model.Order{}).
				Where("id = ? AND status = ?", order.ID, model.OrderStatusRefund).
				Update("status", model.OrderStatusPaid)
			return nil
		}

		// 审批前账户状态快照（P1-4：退款失败转人工时按此恢复）
		prevMPStatus := ""
		if mp != nil {
			prevMPStatus = mp.Status
		}
		// 回收剩余课时：条件更新绑定读取时的 used/total——审批期间有并发消耗则影响行数=0
		reclaimed := 0
		if mp != nil && mp.Status != model.MPStatusRefunded {
			remaining := mp.TotalSessions - mp.UsedSessions
			if remaining > 0 {
				if err := tx.Create(&model.PackageTransaction{
					MemberPackageID: mp.ID, UserID: mp.UserID, SessionsChange: -remaining,
					Type: model.TxnTypeRefundReclaim, IdempotencyKey: "refund_reclaim:" + refund.RefundNo,
					BizType: "refund", BizID: refund.ID, Remark: "退款回收剩余课时",
				}).Error; err != nil {
					return err
				}
			}
			r2 := tx.Model(&model.MemberPackage{}).
				Where("id = ? AND status <> ? AND used_sessions = ? AND total_sessions = ?",
					mp.ID, model.MPStatusRefunded, mp.UsedSessions, mp.TotalSessions).
				Updates(map[string]interface{}{
					"used_sessions": gorm.Expr("total_sessions"), "status": model.MPStatusRefunded,
				})
			if r2.Error != nil {
				return r2.Error
			}
			if r2.RowsAffected == 0 {
				var cur model.MemberPackage
				if err := tx.First(&cur, mp.ID).Error; err == nil && cur.Status == model.MPStatusRefunded {
					remaining = 0 // 已被回收过（幂等重入）
				} else {
					return errs.Conflict("课时余额已变动，请刷新后重新审批")
				}
			}
			reclaimed = remaining
		}
		// 金额与审批快照落库（P0-3：amount 此前从不落库）
		if err := tx.Model(&model.Refund{}).Where("id = ?", refundID).Updates(map[string]interface{}{
			"amount": calc.amount, "unit_price": calc.unitPrice,
			"reclaimed_sessions": reclaimed, "calc_type": calc.calcType,
			"reject_reason": calc.note, "prev_mp_status": prevMPStatus,
		}).Error; err != nil {
			return err
		}
		// 订单累计退款额（订单状态等微信终态再置 refunded）
		return tx.Model(&model.Order{}).Where("id = ?", order.ID).
			UpdateColumn("refund_amount", gorm.Expr("refund_amount + ?", calc.amount)).Error
	})
	if err != nil {
		return nil, err
	}
	if calcErr != nil {
		return nil, calcErr // 补偿写已提交（rejected + 订单回 paid）
	}

	// 2) 事务外发起微信退款（E11：不持锁外呼）。out_refund_no 幂等，可安全重试；
	//    失败不打回本地状态——轮询任务会重试发起并推进终态
	refundErr := s.pay.Refund(ctx, refund.RefundNo, order.OrderNo, calc.amount, order.Amount, refund.Reason)

	// 3) 推进终态（mock QueryRefund 恒 SUCCESS → 即时 refunded；真实通道 PROCESSING → 保持 approved 等轮询）
	if st, qErr := s.pay.QueryRefund(ctx, refund.RefundNo); qErr == nil && st == "SUCCESS" {
		return s.finalizeRefund(ctx, refundID, order.ID)
	}
	if refundErr != nil {
		return nil, errs.Wrap(errs.CodeBiz, "微信退款发起失败，退款单保持处理中，将由任务自动重试", refundErr)
	}
	var out model.Refund
	s.db.WithContext(ctx).First(&out, refundID)
	return &out, nil
}

// PollApprovedRefunds 退款轮询（M4）：处理中的退款单查询微信终态推进；
// 查询报错（多为从未发起成功）则幂等重试发起。mock 通道恒 SUCCESS 即时推进。
func (s *OrderService) PollApprovedRefunds(ctx context.Context) (int, error) {
	var refunds []model.Refund
	if err := s.db.WithContext(ctx).
		Where("status = ?", model.RefundStatusApproved).Limit(50).Find(&refunds).Error; err != nil {
		return 0, err
	}
	done := 0
	for _, r := range refunds {
		// 0 元历史单（P1-2）：已回收课时的恢复后退人工；未回收的回 pending 重新审批，
		// 避免 mock 通道恒 SUCCESS 把 0 元单直接 finalize 卡死
		if r.Amount <= 0 {
			if r.ReclaimedSessions > 0 {
				if err := s.failRefund(ctx, &r, "审批金额为 0 但已回收课时，恢复课时后转人工"); err != nil {
					log.Printf("[refund-poll][ALERT] 0元单恢复失败 %s: %v", r.RefundNo, err)
				} else {
					done++
				}
			} else if err := s.db.WithContext(ctx).Model(&model.Refund{}).
				Where("id = ? AND status = ?", r.ID, model.RefundStatusApproved).
				Update("status", model.RefundStatusPending).Error; err != nil {
				log.Printf("[refund-poll][ALERT] 0元单回退待审失败 %s: %v", r.RefundNo, err)
			} else {
				log.Printf("[refund-poll][ALERT] 0元退款单回退待审 %s", r.RefundNo)
			}
			continue
		}
		st, qErr := s.pay.QueryRefund(ctx, r.RefundNo)
		if qErr != nil {
			// 审批时发起失败的单：out_refund_no 幂等，重试发起
			var order model.Order
			if err := s.db.WithContext(ctx).First(&order, r.OrderID).Error; err == nil {
				if err := s.pay.Refund(ctx, r.RefundNo, order.OrderNo, r.Amount, order.Amount, r.Reason); err != nil {
					log.Printf("[refund-poll] 重新发起退款 %s 失败: %v", r.RefundNo, err)
				}
			}
			continue
		}
		switch st {
		case "SUCCESS":
			if _, err := s.finalizeRefund(ctx, r.ID, r.OrderID); err == nil {
				done++
			} else {
				log.Printf("[refund-poll] 退款 %s 终态落库失败: %v", r.RefundNo, err)
			}
		case "ABNORMAL", "CLOSED":
			// 连续异常达阈值 → failed 转人工（P1-4）
			if err := s.db.WithContext(ctx).Model(&model.Refund{}).
				Where("id = ? AND status = ?", r.ID, model.RefundStatusApproved).
				UpdateColumn("abnormal_count", gorm.Expr("abnormal_count + 1")).Error; err != nil {
				log.Printf("[refund-poll][ALERT] 记录退款异常失败 %s: %v", r.RefundNo, err)
				continue
			}
			var cur model.Refund
			if err := s.db.WithContext(ctx).First(&cur, r.ID).Error; err != nil {
				continue
			}
			if cur.AbnormalCount >= refundAbnormalLimit {
				if err := s.failRefund(ctx, &cur, "微信退款持续异常（"+st+"），转人工处理"); err != nil {
					log.Printf("[refund-poll][ALERT] 退款失败转人工落库失败 %s: %v", r.RefundNo, err)
				} else {
					log.Printf("[refund-poll][ALERT] 退款单 %s 连续异常 %d 次，已置 failed 转人工",
						r.RefundNo, cur.AbnormalCount)
					done++
				}
			}
		default:
			// PROCESSING 等中间态：异常计数清零
			if err := s.db.WithContext(ctx).Model(&model.Refund{}).
				Where("id = ? AND status = ?", r.ID, model.RefundStatusApproved).
				UpdateColumn("abnormal_count", 0).Error; err != nil {
				log.Printf("[refund-poll] 清零退款异常计数失败 %s: %v", r.RefundNo, err)
			}
		}
	}
	return done, nil
}

// failRefund 退款失败转人工（P1-4）：refund approved→failed、订单 refunding→paid、
// 累计退款额回退、按审批前状态恢复课时并写正流水（幂等键 refund_restore:{refundNo}）。
func (s *OrderService) failRefund(ctx context.Context, r *model.Refund, reason string) error {
	var groupBuyID int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Refund{}).
			Where("id = ? AND status = ?", r.ID, model.RefundStatusApproved).
			Updates(map[string]interface{}{
				"status": model.RefundStatusFailed, "reject_reason": reason,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // 已处理（幂等）
		}
		var order model.Order
		if err := tx.First(&order, r.OrderID).Error; err != nil {
			return err
		}
		groupBuyID = order.GroupBuyID
		ores := tx.Model(&model.Order{}).
			Where("id = ? AND status = ?", r.OrderID, model.OrderStatusRefund).
			Update("status", model.OrderStatusPaid)
		if ores.Error != nil {
			return ores.Error
		}
		if ores.RowsAffected == 0 {
			return nil // 订单已非 refunding：仅退款单转人工
		}
		// 回退累计退款额
		if r.Amount > 0 {
			if err := tx.Model(&model.Order{}).Where("id = ?", r.OrderID).
				UpdateColumn("refund_amount", gorm.Expr(
					"CASE WHEN refund_amount >= ? THEN refund_amount - ? ELSE 0 END",
					r.Amount, r.Amount)).Error; err != nil {
				return err
			}
		}
		// 恢复课时账户
		if r.ReclaimedSessions > 0 {
			var mp model.MemberPackage
			if err := tx.Where("order_id = ?", r.OrderID).First(&mp).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil
				}
				return err
			}
			status := r.PrevMPStatus
			if status == "" || status == model.MPStatusRefunded {
				status = model.MPStatusActive
			}
			if err := tx.Model(&model.MemberPackage{}).Where("id = ?", mp.ID).
				Updates(map[string]interface{}{
					"used_sessions": gorm.Expr(
						"CASE WHEN used_sessions >= ? THEN used_sessions - ? ELSE 0 END",
						r.ReclaimedSessions, r.ReclaimedSessions),
					"status": status,
				}).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.PackageTransaction{
				MemberPackageID: mp.ID, UserID: mp.UserID, SessionsChange: r.ReclaimedSessions,
				Type: model.TxnTypeRefundRestore, IdempotencyKey: "refund_restore:" + r.RefundNo,
				BizType: "refund", BizID: r.ID, Remark: "退款失败恢复课时（转人工）",
			}).Error; err != nil && !isDuplicateKey(err) {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 拼团单回 paid：尝试成团（幂等）
	if groupBuyID > 0 && s.groupBuy != nil {
		s.groupBuy.OnOrderPaid(ctx, r.OrderID)
	}
	return nil
}

// finalizeRefund 退款终态：refund approved → refunded、订单 refunding → refunded、销量回退。
// 两处状态翻转均以 RowsAffected 为幂等闸门：审批路径与轮询并发时只生效一次，
// 不重复扣销量/返券（P1-4 新发现）。
func (s *OrderService) finalizeRefund(ctx context.Context, refundID, orderID int64) (*model.Refund, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rres := tx.Model(&model.Refund{}).
			Where("id = ? AND status = ?", refundID, model.RefundStatusApproved).
			Update("status", model.RefundStatusRefunded)
		if rres.Error != nil {
			return rres.Error
		}
		if rres.RowsAffected == 0 {
			return nil // 已终态（并发路径已处理），幂等返回
		}
		var order model.Order
		if err := tx.First(&order, orderID).Error; err != nil {
			return err
		}
		ores := tx.Model(&model.Order{}).
			Where("id = ? AND status = ?", orderID, model.OrderStatusRefund).
			Update("status", model.OrderStatusRefunded)
		if ores.Error != nil {
			return ores.Error
		}
		if ores.RowsAffected == 0 {
			return nil // 订单已被并发推进终态：不再扣销量/返券
		}
		// 用券订单退款：券在有效期内自动返还（usable），过期置 expired 留痕（规划 §7.1）
		if err := returnCouponOnRefund(tx, &order); err != nil {
			return err
		}
		// 展示口径：退款回退销量（CASE 兼容 sqlite，不用 GREATEST）
		if err := tx.Model(&model.CoursePackage{}).Where("id = ?", order.PackageID).
			UpdateColumn("sold_count", gorm.Expr(
				"CASE WHEN sold_count > 0 THEN sold_count - 1 ELSE 0 END")).Error; err != nil {
			return err
		}
		// 拼团单退款终态：释放占席（团仍在报名中时）；成员行保留，uk 防重复参团（P0-1）
		if order.GroupBuyID > 0 {
			if err := tx.Model(&model.GroupBuy{}).
				Where("id = ? AND status = ?", order.GroupBuyID, model.GroupBuyEnrolling).
				UpdateColumn("joined_count", gorm.Expr(
					"CASE WHEN joined_count > 0 THEN joined_count - 1 ELSE 0 END")).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, errs.Server(err, "退款终态落库失败")
	}
	var out model.Refund
	s.db.WithContext(ctx).First(&out, refundID)
	return &out, nil
}

// RejectRefund 后台拒绝退款：退款单 rejected、订单回 paid。
// 两条 UPDATE 同事务并检查影响行数（P1-3）：DB 故障不再被误报为「已被处理」。
func (s *OrderService) RejectRefund(ctx context.Context, refundID, adminID int64, reason string) error {
	var refund model.Refund
	if err := s.db.WithContext(ctx).First(&refund, refundID).Error; err != nil {
		return errs.NotFound("退款单不存在")
	}
	var groupBuyID int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Refund{}).
			Where("id = ? AND status = ?", refundID, model.RefundStatusPending).
			Updates(map[string]interface{}{"status": model.RefundStatusRejected, "reject_reason": reason, "admin_id": adminID})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errs.Conflict("该退款单已被处理")
		}
		ores := tx.Model(&model.Order{}).
			Where("id = ? AND status = ?", refund.OrderID, model.OrderStatusRefund).
			Update("status", model.OrderStatusPaid)
		if ores.Error != nil {
			return ores.Error
		}
		if ores.RowsAffected == 0 {
			var cur model.Order
			if err := tx.First(&cur, refund.OrderID).Error; err != nil {
				return errs.Server(err, "查询订单失败")
			}
			if cur.Status == model.OrderStatusRefunded {
				return errs.Conflict("订单已退款完成，无法拒绝")
			}
			// 已 paid：幂等
		}
		var cur model.Order
		if err := tx.Select("group_buy_id").First(&cur, refund.OrderID).Error; err == nil {
			groupBuyID = cur.GroupBuyID
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 拼团单回 paid：尝试成团（幂等）
	if groupBuyID > 0 && s.groupBuy != nil {
		s.groupBuy.OnOrderPaid(ctx, refund.OrderID)
	}
	return nil
}

// RepairRefundingOrders 修复孤儿退款态（P1-3）：订单 refunding 但无 pending/approved 退款单。
// 有 refunded 退款单 → 补订单终态；否则订单回 paid（拼团单触发成团检查），转人工核对。
func (s *OrderService) RepairRefundingOrders(ctx context.Context) (int, error) {
	var orders []model.Order
	if err := s.db.WithContext(ctx).
		Where("status = ?", model.OrderStatusRefund).
		Where("NOT EXISTS (SELECT 1 FROM refund r WHERE r.order_id = orders.id AND r.status IN ?)",
			[]string{model.RefundStatusPending, model.RefundStatusApproved}).
		Limit(100).Find(&orders).Error; err != nil {
		return 0, err
	}
	fixed := 0
	for _, o := range orders {
		var last model.Refund
		err := s.db.WithContext(ctx).Where("order_id = ?", o.ID).Order("id DESC").First(&last).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[repair] 查询订单 %d 退款单失败: %v", o.ID, err)
			continue
		}
		if err == nil && last.Status == model.RefundStatusRefunded {
			// 退款已成功但订单未落终态：补终态（销量是否已扣无法确定，告警人工核对）
			res := s.db.WithContext(ctx).Model(&model.Order{}).
				Where("id = ? AND status = ?", o.ID, model.OrderStatusRefund).
				Update("status", model.OrderStatusRefunded)
			if res.Error != nil {
				log.Printf("[repair][ALERT] 订单 %d 补退款终态失败: %v", o.ID, res.Error)
				continue
			}
			if res.RowsAffected > 0 {
				fixed++
				log.Printf("[repair][ALERT] 订单 %d 补置 refunded，请核对销量/券返还", o.ID)
			}
			continue
		}
		// 无退款单或退款单 rejected/failed：订单回 paid
		res := s.db.WithContext(ctx).Model(&model.Order{}).
			Where("id = ? AND status = ?", o.ID, model.OrderStatusRefund).
			Update("status", model.OrderStatusPaid)
		if res.Error != nil {
			log.Printf("[repair][ALERT] 订单 %d 回 paid 失败: %v", o.ID, res.Error)
			continue
		}
		if res.RowsAffected > 0 {
			fixed++
			log.Printf("[repair][ALERT] 订单 %d 无活跃退款单，已回 paid 待人工核对", o.ID)
			if o.GroupBuyID > 0 && s.groupBuy != nil {
				s.groupBuy.OnOrderPaid(ctx, o.ID)
			}
		}
	}
	return fixed, nil
}

// HandlePaySuccess 支付成功统一入口（mock 直调 / 真实回调验签后调）。
// 每一步都以条件 UPDATE 或唯一键做幂等，重复回调安全。
// paidAmount 为回调携带的实付金额（M1）：与订单不符时落失败流水并拒绝；
// 关单在先的迟到回调幂等应答（M2），避免微信无限重试。
func (s *OrderService) HandlePaySuccess(ctx context.Context, orderNo, transactionID, raw string, paidAmount int64) error {
	var rejectErr error // 事务内留痕后需要向调用方报错时置值（回调里 return err 会回滚留痕）
	var lateGroup bool  // 拼团已结束的迟到付款：不入账，转自动退款（P0-1）
	var order model.Order
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return errs.NotFound("订单不存在")
		}
		if order.Status == model.OrderStatusPaid || order.Status == model.OrderStatusRefund ||
			order.Status == model.OrderStatusRefunded {
			return nil // 已处理，幂等返回
		}
		if paidAmount > 0 && paidAmount != order.Amount {
			markPaymentFailed(tx, order.ID, raw)
			log.Printf("[pay] 回调金额不符：order=%s expect=%d got=%d", orderNo, order.Amount, paidAmount)
			rejectErr = errs.Biz("回调金额与订单不符")
			return nil // 提交留痕，错误经 rejectErr 返回
		}
		if order.Status == model.OrderStatusClosed {
			// 超时关单在先的迟到支付：钱停在微信侧待人工处理，先幂等应答止血
			markPaymentFailed(tx, order.ID, raw)
			log.Printf("[pay] 迟到回调命中已关单：order=%s txn=%s", orderNo, transactionID)
			return nil
		}
		if order.Status != model.OrderStatusPending {
			return errs.Biz("订单状态异常，无法入账")
		}
		now := time.Now()
		// 订单 pending → paid（条件更新抢闸门）
		res := tx.Model(&model.Order{}).
			Where("id = ? AND status = ?", order.ID, model.OrderStatusPending).
			Updates(map[string]interface{}{
				"status": model.OrderStatusPaid, "pay_time": now, "transaction_id": transactionID,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// 并发回调已抢先处理，或回调与超时关单竞态：回读区分并留痕（P1-1）
			var cur model.Order
			if err := tx.First(&cur, order.ID).Error; err == nil && cur.Status == model.OrderStatusClosed {
				markPaymentFailed(tx, order.ID, raw)
				log.Printf("[pay][ALERT] 迟到回调命中已关单（已留痕）：order=%s txn=%s", orderNo, transactionID)
			}
			return nil
		}
		// 支付流水 created → success（transaction_id 唯一键防重放）
		payRes := tx.Model(&model.Payment{}).
			Where("order_id = ? AND status = ?", order.ID, "created").
			Updates(map[string]interface{}{
				"transaction_id": transactionID, "status": "success", "raw_notify": raw,
			})
		if payRes.Error != nil {
			return payRes.Error
		}
		if payRes.RowsAffected == 0 {
			// 支付单缺失不阻断入账（订单侧闸门已保证幂等），仅告警（M11）
			log.Printf("[pay] 订单 %s 支付单缺失，跳过流水更新", orderNo)
		}
		// 课时入账前校验拼团状态（P0-1）：团已失败/结束的迟到付款不入账，事务外转自动退款。
		// MySQL 下 FOR UPDATE 锁团行，与 CloseGroupBuys 的条件 UPDATE 互斥，消除竞态；
		// sqlite（测试）不支持 FOR UPDATE，跳过锁由条件更新路径兜底。
		if order.GroupBuyID > 0 {
			q := tx
			if tx.Dialector.Name() == "mysql" {
				q = q.Clauses(clause.Locking{Strength: "UPDATE"})
			}
			var gb model.GroupBuy
			if err := q.First(&gb, order.GroupBuyID).Error; err != nil {
				return errs.Server(err, "查询拼团状态失败")
			}
			if gb.Status != model.GroupBuyEnrolling && gb.Status != model.GroupBuySuccess {
				lateGroup = true
				return nil
			}
		}
		// 课时入账：member_package（按订单幂等）+ 购入流水（唯一键幂等）
		return creditSessions(tx, &order, now)
	})
	if err != nil {
		return err
	}
	if rejectErr != nil {
		return rejectErr
	}
	if lateGroup {
		log.Printf("[pay][ALERT] 拼团已结束的迟到付款，转自动退款：order=%s gb=%d", orderNo, order.GroupBuyID)
		refund, rerr := s.CreateAutoRefund(ctx, order.ID, order.UserID, "拼团已结束，迟到支付自动退款")
		if rerr != nil {
			log.Printf("[pay][ALERT] 迟到付款建退款单失败 order=%s: %v", orderNo, rerr)
			return nil
		}
		if _, aerr := s.ApproveRefund(ctx, refund.ID, 0); aerr != nil {
			log.Printf("[pay] 迟到付款自动退款审批失败 order=%s refund=%s: %v", orderNo, refund.RefundNo, aerr)
		}
		return nil
	}
	// 事务外联动：老带新首单奖励（自身幂等，失败不影响支付结果）
	if s.referral != nil {
		s.referral.RewardOnPaidOrder(ctx, order.UserID)
	}
	// 事务外联动：拼团单尝试成团（幂等）
	if s.groupBuy != nil {
		s.groupBuy.OnOrderPaid(ctx, order.ID)
	}
	return nil
}

// markPaymentFailed 支付失败/金额不符/迟到回调的流水留痕（幂等）
func markPaymentFailed(tx *gorm.DB, orderID int64, raw string) {
	tx.Model(&model.Payment{}).
		Where("order_id = ? AND status = ?", orderID, "created").
		Updates(map[string]interface{}{"status": "failed", "raw_notify": raw})
}

// MarkPayFailedByOrderNo 非成功回调（CLOSED/PAYERROR 等）的流水留痕（幂等）
func (s *OrderService) MarkPayFailedByOrderNo(ctx context.Context, orderNo, raw string) {
	var order model.Order
	if err := s.db.WithContext(ctx).Select("id").Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return
	}
	s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		markPaymentFailed(tx, order.ID, raw)
		return nil
	})
}

// creditSessions 支付成功后的课时入账（独立函数便于单测）。
// 一律以订单 SKU 快照为准（M5）：后台删/改规格不影响已支付订单；
// 仅 00005 回填前的老订单快照缺失时才回查 SKU 现值兜底。
func creditSessions(tx *gorm.DB, order *model.Order, now time.Time) error {
	var exist model.MemberPackage
	err := tx.Where("order_id = ?", order.ID).First(&exist).Error
	if err == nil {
		return nil // 已入账
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	sessions, validDays, unitPrice := order.SkuSessions, order.SkuValidDays, order.SkuUnitPrice
	if sessions == 0 {
		var sku model.PackageSKU
		if err := tx.First(&sku, order.SkuID).Error; err == nil && sku.Sessions > 0 {
			sessions = sku.Sessions
			validDays = sku.ValidDays
			unitPrice = sku.Price / int64(sku.Sessions)
		}
	}
	if sessions == 0 {
		log.Printf("[order] 订单 %s 无课时快照且 SKU 已不可用，建 0 课时账户待人工处理", order.OrderNo)
	}
	mp := model.MemberPackage{
		OwnerID: order.OwnerID, UserID: order.UserID, PackageID: order.PackageID,
		SkuID: order.SkuID, OrderID: order.ID, PackageTitle: order.Title,
		TotalSessions: sessions + order.GiftSessions, GiftSessions: order.GiftSessions,
		ValidDays: validDays, UnitPrice: unitPrice,
		Status: model.MPStatusInactive,
	}
	if err := tx.Create(&mp).Error; err != nil {
		return err
	}
	if err := tx.Create(&model.PackageTransaction{
		MemberPackageID: mp.ID, UserID: order.UserID, SessionsChange: sessions,
		Type: model.TxnTypePurchase, IdempotencyKey: "purchase:" + order.OrderNo,
		BizType: "order", BizID: order.ID, Remark: "购买入账",
	}).Error; err != nil {
		return err
	}
	// 赠课券赠送课时：独立流水（唯一键幂等），不与购入混在一条便于对账
	if order.GiftSessions > 0 {
		if err := tx.Create(&model.PackageTransaction{
			MemberPackageID: mp.ID, UserID: order.UserID, SessionsChange: order.GiftSessions,
			Type: model.TxnTypeGift, IdempotencyKey: "gift:" + order.OrderNo,
			BizType: "order", BizID: order.ID, Remark: "赠课券赠送",
		}).Error; err != nil {
			return err
		}
	}
	return tx.Model(&model.CoursePackage{}).Where("id = ?", order.PackageID).
		UpdateColumn("sold_count", gorm.Expr("sold_count + 1")).Error
}

func (s *OrderService) getOwnedOrder(ctx context.Context, orderID, userID int64) (*model.Order, error) {
	var order model.Order
	err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error
	if err != nil {
		return nil, errs.NotFound("订单不存在")
	}
	return &order, nil
}
