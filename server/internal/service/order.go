package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
	"fitness/internal/wechat"
)

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
		OrderNo:   genOrderNo(),
		OwnerID:   pkg.OwnerID,
		UserID:    userID,
		PackageID: pkg.ID,
		SkuID:     sku.ID,
		Title:     fmt.Sprintf("%s·%s", pkg.Title, sku.Name),
		Amount:    amount,
		CouponID:  applyCoupon,
		CouponAmount: couponAmount,
		Status:    model.OrderStatusPending,
		SkuSessions:   sku.Sessions,
		SkuValidDays:  sku.ValidDays,
		SkuUnitPrice:  unitPrice,
		NewcomerUsed:  newcomerUsed,
		PromoUsed:     promoUsed,
		GiftSessions:  giftAmount,
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
// mock 模式：直接把该单置为已支付（走 HandlePaySuccess 幂等链路）。
func (s *OrderService) Prepay(ctx context.Context, orderID, userID int64) (*wechat.PrepayResult, error) {
	order, err := s.getOwnedOrder(ctx, orderID, userID)
	if err != nil {
		return nil, err
	}
	if order.Status != model.OrderStatusPending {
		return nil, errs.Biz("订单当前状态不可支付")
	}
	var user model.User
	if err := s.db.WithContext(ctx).Select("id", "openid").First(&user, userID).Error; err != nil {
		return nil, errs.NotFound("用户不存在")
	}
	res, err := s.pay.Prepay(ctx, order.OrderNo, user.OpenID, order.Amount, order.Title)
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

// MyOrders 我的订单（倒序分页）
func (s *OrderService) MyOrders(ctx context.Context, userID int64, page, pageSize int) ([]model.Order, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	q := s.db.WithContext(ctx).Model(&model.Order{}).Where("user_id = ?", userID)
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
func (s *OrderService) CreateAutoRefund(ctx context.Context, orderID, userID int64, reason string) (*model.Refund, error) {
	refund := model.Refund{
		RefundNo: genRefundNo(), OrderID: orderID, UserID: userID,
		Reason: reason, Status: model.RefundStatusPending,
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
// 否则按「剩余课时 × 单课时快照价」折算。以调用时的最新余额为准（事务内重读）。
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
	// 赠课不参与退款折算：可退课时 = max(0, 总课时 - 赠课 - 已消耗)
	remaining := mp.TotalSessions - mp.UsedSessions - mp.GiftSessions
	if remaining < 0 {
		remaining = 0
	}
	if remaining <= 0 {
		return refundCalc{note: "剩余课时均为赠课或已用完，无可退金额"}, errs.Biz("课时已用完，无可退金额")
	}
	amount := int64(remaining) * mp.UnitPrice
	if amount > order.Amount-order.RefundAmount {
		amount = order.Amount - order.RefundAmount // 折算价不得高于实付余额
	}
	note := fmt.Sprintf("剩余可退 %d 课时 × %d 分/课时", remaining, mp.UnitPrice)
	if mp.GiftSessions > 0 {
		note += fmt.Sprintf("（%d 节赠课不参与退款）", mp.GiftSessions)
	}
	return refundCalc{
		amount:    amount,
		note:      note,
		calcType:  "remainder",
		unitPrice: mp.UnitPrice,
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
	// 1) 抢闸门：pending → approved
	res := s.db.WithContext(ctx).Model(&model.Refund{}).
		Where("id = ? AND status = ?", refundID, model.RefundStatusPending).
		Updates(map[string]interface{}{"status": model.RefundStatusApproved, "admin_id": adminID})
	if res.RowsAffected == 0 {
		return nil, errs.Conflict("该退款单已被处理")
	}

	// 2) 事务：重读重算 + 快照落库 + 回收课时
	var order model.Order
	var calc refundCalc
	var calcErr error
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
			"reclaimed_sessions": reclaimed, "calc_type": calc.calcType, "reject_reason": calc.note,
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

	// 3) 事务外发起微信退款（E11：不持锁外呼）。out_refund_no 幂等，可安全重试；
	//    失败不打回本地状态——轮询任务会重试发起并推进终态
	refundErr := s.pay.Refund(ctx, refund.RefundNo, order.OrderNo, calc.amount, order.Amount, refund.Reason)

	// 4) 推进终态（mock QueryRefund 恒 SUCCESS → 即时 refunded；真实通道 PROCESSING → 保持 approved 等轮询）
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
		st, qErr := s.pay.QueryRefund(ctx, r.RefundNo)
		if qErr != nil {
			// 审批时发起失败的单：out_refund_no 幂等，重试发起
			var order model.Order
			if err := s.db.WithContext(ctx).First(&order, r.OrderID).Error; err == nil && r.Amount > 0 {
				if err := s.pay.Refund(ctx, r.RefundNo, order.OrderNo, r.Amount, order.Amount, r.Reason); err != nil {
					log.Printf("[refund-poll] 重新发起退款 %s 失败: %v", r.RefundNo, err)
				}
			}
			continue
		}
		if st == "SUCCESS" {
			if _, err := s.finalizeRefund(ctx, r.ID, r.OrderID); err == nil {
				done++
			} else {
				log.Printf("[refund-poll] 退款 %s 终态落库失败: %v", r.RefundNo, err)
			}
		}
	}
	return done, nil
}

// finalizeRefund 退款终态：refund approved → refunded、订单 refunding → refunded、销量回退
func (s *OrderService) finalizeRefund(ctx context.Context, refundID, orderID int64) (*model.Refund, error) {	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Refund{}).
			Where("id = ? AND status = ?", refundID, model.RefundStatusApproved).
			Update("status", model.RefundStatusRefunded).Error; err != nil {
			return err
		}
		var order model.Order
		if err := tx.First(&order, orderID).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Order{}).
			Where("id = ? AND status = ?", orderID, model.OrderStatusRefund).
			Update("status", model.OrderStatusRefunded).Error; err != nil {
			return err
		}
		// 用券订单退款：券在有效期内自动返还（usable），过期置 expired 留痕（规划 §7.1）
		if err := returnCouponOnRefund(tx, &order); err != nil {
			return err
		}
		// 展示口径：退款回退销量（CASE 兼容 sqlite，不用 GREATEST）
		return tx.Model(&model.CoursePackage{}).Where("id = ?", order.PackageID).
			UpdateColumn("sold_count", gorm.Expr(
				"CASE WHEN sold_count > 0 THEN sold_count - 1 ELSE 0 END")).Error
	})
	if err != nil {
		return nil, errs.Server(err, "退款终态落库失败")
	}
	var out model.Refund
	s.db.WithContext(ctx).First(&out, refundID)
	return &out, nil
}

// RejectRefund 后台拒绝退款：退款单 rejected、订单回 paid
func (s *OrderService) RejectRefund(ctx context.Context, refundID, adminID int64, reason string) error {
	var refund model.Refund
	if err := s.db.WithContext(ctx).First(&refund, refundID).Error; err != nil {
		return errs.NotFound("退款单不存在")
	}
	res := s.db.WithContext(ctx).Model(&model.Refund{}).
		Where("id = ? AND status = ?", refundID, model.RefundStatusPending).
		Updates(map[string]interface{}{"status": model.RefundStatusRejected, "reject_reason": reason, "admin_id": adminID})
	if res.RowsAffected == 0 {
		return errs.Conflict("该退款单已被处理")
	}
	s.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND status = ?", refund.OrderID, model.OrderStatusRefund).
		Update("status", model.OrderStatusPaid)
	return nil
}

// HandlePaySuccess 支付成功统一入口（mock 直调 / 真实回调验签后调）。
// 每一步都以条件 UPDATE 或唯一键做幂等，重复回调安全。
// paidAmount 为回调携带的实付金额（M1）：与订单不符时落失败流水并拒绝；
// 关单在先的迟到回调幂等应答（M2），避免微信无限重试。
func (s *OrderService) HandlePaySuccess(ctx context.Context, orderNo, transactionID, raw string, paidAmount int64) error {
	var rejectErr error // 事务内留痕后需要向调用方报错时置值（回调里 return err 会回滚留痕）
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
			return nil // 并发回调另一条已抢先处理
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
		// 课时入账：member_package（按订单幂等）+ 购入流水（唯一键幂等）
		return creditSessions(tx, &order, now)
	})
	if err != nil {
		return err
	}
	if rejectErr != nil {
		return rejectErr
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
