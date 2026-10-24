package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
	"fitness/internal/wechat"
)

// OrderService 订单状态机：pending → paid →（refunding → refunded / closed）。
// 支付成功回调（真实或 mock）统一走 HandlePaySuccess，全程幂等。
type OrderService struct {
	db   *gorm.DB
	cfg  *ConfigService
	pay  wechat.PaymentProvider
}

func NewOrderService(db *gorm.DB, cfg *ConfigService, pay wechat.PaymentProvider) *OrderService {
	return &OrderService{db: db, cfg: cfg, pay: pay}
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

// Create 下单（P0 coupon 必须为 0，P1 接券系统）
func (s *OrderService) Create(ctx context.Context, userID int64, skuID int64, couponID int64) (*model.Order, error) {
	if couponID != 0 {
		return nil, errs.Biz("优惠券功能即将上线，敬请期待")
	}
	var sku model.PackageSKU
	if err := s.db.WithContext(ctx).Where("id = ? AND status = 1", skuID).First(&sku).Error; err != nil {
		return nil, errs.NotFound("课包规格不存在或已停售")
	}
	var pkg model.CoursePackage
	if err := s.db.WithContext(ctx).Where("id = ? AND status = 1", sku.PackageID).First(&pkg).Error; err != nil {
		return nil, errs.NotFound("课包不存在或已下架")
	}
	order := model.Order{
		OrderNo:   genOrderNo(),
		OwnerID:   pkg.OwnerID,
		UserID:    userID,
		PackageID: pkg.ID,
		SkuID:     sku.ID,
		Title:     fmt.Sprintf("%s·%s", pkg.Title, sku.Name),
		Amount:    sku.Price,
		Status:    model.OrderStatusPending,
	}
	if err := s.db.WithContext(ctx).Create(&order).Error; err != nil {
		return nil, errs.Server(err, "创建订单失败")
	}
	if err := s.db.WithContext(ctx).Create(&model.Payment{
		OrderID: order.ID, Amount: order.Amount, Status: "created",
	}).Error; err != nil {
		return nil, errs.Server(err, "创建支付单失败")
	}
	return &order, nil
}

// Prepay 发起支付。mock 模式：直接把该单置为已支付（走 HandlePaySuccess 幂等链路）。
func (s *OrderService) Prepay(ctx context.Context, orderID, userID int64, openid string) (*wechat.PrepayResult, error) {
	order, err := s.getOwnedOrder(ctx, orderID, userID)
	if err != nil {
		return nil, err
	}
	if order.Status != model.OrderStatusPending {
		return nil, errs.Biz("订单当前状态不可支付")
	}
	res, err := s.pay.Prepay(ctx, order.OrderNo, openid, order.Amount, order.Title)
	if err != nil {
		return nil, errs.Wrap(errs.CodeBiz, "发起支付失败", err)
	}
	if res.MockPaid {
		// 模拟微信回调：直接进入支付成功链路（幂等，重复调用安全）
		if err := s.HandlePaySuccess(ctx, order.OrderNo, "mocktxn-"+order.OrderNo, "mock"); err != nil {
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

// ApplyRefund 学员申请退款：仅已支付订单；金额由后台审核时按规则计算
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
	// 状态闸门：paid → refunding（并发防重）
	res := s.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND status = ?", orderID, model.OrderStatusPaid).
		Update("status", model.OrderStatusRefund)
	if res.RowsAffected == 0 {
		return nil, errs.Conflict("重复提交，请刷新查看")
	}
	refund := model.Refund{
		RefundNo: genRefundNo(), OrderID: order.ID, UserID: userID,
		Reason: reason, Status: model.RefundStatusPending,
	}
	if err := s.db.WithContext(ctx).Create(&refund).Error; err != nil {
		return nil, errs.Server(err, "创建退款单失败")
	}
	return &refund, nil
}

// calcRefundAmount 退款金额规则：
// 未开卡且支付未超过 refund.full_days 天 → 全额；
// 否则按「剩余课时 × 单课时快照价」折算（未开卡剩余=全部课时）。
func (s *OrderService) calcRefundAmount(ctx context.Context, order *model.Order, mp *model.MemberPackage) (int64, string, error) {
	fullDays := s.cfg.GetInt(ctx, model.CfgRefundFullDays, 7)
	if mp == nil {
		return order.Amount, "无课时账户，全额退", nil
	}
	if mp.Status == model.MPStatusInactive && order.PayTime != nil &&
		time.Since(*order.PayTime) <= time.Duration(fullDays)*24*time.Hour {
		return order.Amount, fmt.Sprintf("未开卡且在 %d 天内，全额退", fullDays), nil
	}
	remaining := mp.TotalSessions - mp.UsedSessions
	if remaining <= 0 {
		return 0, "课时已用完，无可退金额", errs.Biz("课时已用完，无可退金额")
	}
	amount := int64(remaining) * mp.UnitPrice
	if amount > order.Amount-order.RefundAmount {
		amount = order.Amount - order.RefundAmount // 折算价不得高于实付余额
	}
	return amount, fmt.Sprintf("剩余 %d 课时 × %d 分/课时", remaining, mp.UnitPrice), nil
}

// ApproveRefund 后台同意退款：计算金额 → 回收课时 → 订单 refunded → 退款单 refunded（mock 即时到账）
func (s *OrderService) ApproveRefund(ctx context.Context, refundID, adminID int64) (*model.Refund, error) {
	var refund model.Refund
	if err := s.db.WithContext(ctx).First(&refund, refundID).Error; err != nil {
		return nil, errs.NotFound("退款单不存在")
	}
	var order model.Order
	if err := s.db.WithContext(ctx).First(&order, refund.OrderID).Error; err != nil {
		return nil, errs.NotFound("订单不存在")
	}
	var mp model.MemberPackage
	hasMP := s.db.WithContext(ctx).Where("order_id = ?", order.ID).First(&mp).Error == nil

	amount, note, err := s.calcRefundAmount(ctx, &order, &mp)
	if err != nil {
		// 无可退金额：回滚为拒绝
		s.db.WithContext(ctx).Model(&model.Refund{}).Where("id = ?", refundID).
			Updates(map[string]interface{}{"status": model.RefundStatusRejected, "reject_reason": note})
		return nil, err
	}

	// 微信侧先发起退款（out_refund_no 幂等，重试安全）；失败保持 pending 供重试
	if err := s.pay.Refund(ctx, refund.RefundNo, order.OrderNo, amount, order.Amount, refund.Reason); err != nil {
		return nil, errs.Wrap(errs.CodeBiz, "微信退款发起失败，请稍后重试", err)
	}

	// 状态闸门 pending → approved
	res := s.db.WithContext(ctx).Model(&model.Refund{}).
		Where("id = ? AND status = ?", refundID, model.RefundStatusPending).
		Updates(map[string]interface{}{"status": model.RefundStatusApproved, "admin_id": adminID})
	if res.RowsAffected == 0 {
		return nil, errs.Conflict("该退款单已被处理")
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 回收剩余课时（幂等键 per 退款单）
		if hasMP && mp.Status != model.MPStatusRefunded {
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
			if err := tx.Model(&model.MemberPackage{}).Where("id = ?", mp.ID).
				Updates(map[string]interface{}{
					"used_sessions": gorm.Expr("total_sessions"), "status": model.MPStatusRefunded,
				}).Error; err != nil {
				return err
			}
		}
		// 订单 refunded + 累计退款额
		if err := tx.Model(&model.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
			"status": model.OrderStatusRefunded, "refund_amount": gorm.Expr("refund_amount + ?", amount),
		}).Error; err != nil {
			return err
		}
		// mock 通道即时到账；真实通道 PROCESSING 等，由退款回调/查询推进
		finalStatus := model.RefundStatusRefunded
		if st, err := s.pay.QueryRefund(ctx, refund.RefundNo); err == nil && st != "SUCCESS" {
			finalStatus = model.RefundStatusApproved
		}
		return tx.Model(&model.Refund{}).Where("id = ?", refundID).
			Update("status", finalStatus).Error
	})
	if err != nil {
		return nil, errs.Server(err, "退款执行失败")
	}
	refund.Amount = amount
	refund.Status = model.RefundStatusRefunded
	return &refund, nil
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
func (s *OrderService) HandlePaySuccess(ctx context.Context, orderNo, transactionID, raw string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order model.Order
		if err := tx.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return errs.NotFound("订单不存在")
		}
		if order.Status == model.OrderStatusPaid || order.Status == model.OrderStatusRefund ||
			order.Status == model.OrderStatusRefunded {
			return nil // 已处理，幂等返回
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
		if err := tx.Model(&model.Payment{}).
			Where("order_id = ? AND status = ?", order.ID, "created").
			Updates(map[string]interface{}{
				"transaction_id": transactionID, "status": "success", "raw_notify": raw,
			}).Error; err != nil {
			return err
		}
		// 课时入账：member_package（按订单幂等）+ 购入流水（唯一键幂等）
		return creditSessions(tx, &order, now)
	})
}

// creditSessions 支付成功后的课时入账（独立函数便于单测）
func creditSessions(tx *gorm.DB, order *model.Order, now time.Time) error {
	var exist model.MemberPackage
	err := tx.Where("order_id = ?", order.ID).First(&exist).Error
	if err == nil {
		return nil // 已入账
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var sku model.PackageSKU
	if err := tx.First(&sku, order.SkuID).Error; err != nil {
		// 规格被删时兜底：仅按订单金额建账，单课时价 0（退款按剩余比例）
		return tx.Create(&model.MemberPackage{
			OwnerID: order.OwnerID, UserID: order.UserID, PackageID: order.PackageID,
			SkuID: order.SkuID, OrderID: order.ID, PackageTitle: order.Title,
			TotalSessions: 0, Status: model.MPStatusInactive,
		}).Error
	}
	unitPrice := int64(0)
	if sku.Sessions > 0 {
		unitPrice = sku.Price / int64(sku.Sessions)
	}
	mp := model.MemberPackage{
		OwnerID: order.OwnerID, UserID: order.UserID, PackageID: order.PackageID,
		SkuID: order.SkuID, OrderID: order.ID, PackageTitle: order.Title,
		TotalSessions: sku.Sessions, ValidDays: sku.ValidDays, UnitPrice: unitPrice,
		Status: model.MPStatusInactive,
	}
	if err := tx.Create(&mp).Error; err != nil {
		return err
	}
	if err := tx.Create(&model.PackageTransaction{
		MemberPackageID: mp.ID, UserID: order.UserID, SessionsChange: sku.Sessions,
		Type: model.TxnTypePurchase, IdempotencyKey: "purchase:" + order.OrderNo,
		BizType: "order", BizID: order.ID, Remark: "购买入账",
	}).Error; err != nil {
		return err
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
