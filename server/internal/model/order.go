package model

import "time"

// 订单状态机：pending → paid → refunding → refunded；pending → closed（超时/取消）
const (
	OrderStatusPending  = "pending"
	OrderStatusPaid     = "paid"
	OrderStatusClosed   = "closed"
	OrderStatusRefund   = "refunding"
	OrderStatusRefunded = "refunded"
)

// Order 订单（coupon 字段 P1 预留，P0 恒为 0）
type Order struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderNo       string    `gorm:"column:order_no;size:32;uniqueIndex" json:"orderNo"`
	OwnerID       int64     `gorm:"column:owner_id;default:1" json:"-"`
	UserID        int64     `gorm:"index:idx_order_user" json:"userId"`
	PackageID     int64     `gorm:"index:idx_order_pkg" json:"packageId"`
	SkuID         int64     `json:"skuId"`
	Title         string    `gorm:"size:128" json:"title"`
	Amount        int64     `json:"amount"`       // 实付（分）
	CouponID      int64     `gorm:"default:0" json:"couponId"`
	CouponAmount  int64     `gorm:"default:0" json:"couponAmount"`
	Status        string    `gorm:"size:16;default:pending;index:idx_order_user" json:"status"`
	PayTime       *time.Time `json:"payTime"`
	TransactionID string    `gorm:"column:transaction_id;size:64" json:"transactionId"`
	RefundAmount  int64     `gorm:"default:0" json:"refundAmount"`
	// 下单时 SKU 快照：后台删/改规格后，入账与退款仍以本单为准（M5）
	SkuSessions   int       `gorm:"default:0" json:"skuSessions"`
	SkuValidDays  int       `gorm:"default:0" json:"skuValidDays"`
	SkuUnitPrice  int64     `gorm:"default:0" json:"skuUnitPrice"`
	// 本单使用了新人价（每人限购一次，按手机号判重；与优惠券不叠加取低价）
	NewcomerUsed  int8      `gorm:"default:0" json:"newcomerUsed"`
	PromoUsed     int8      `gorm:"default:0" json:"promoUsed"`    // 本单使用了限时活动价
	GiftSessions  int       `gorm:"default:0" json:"giftSessions"` // 赠课券赠送课时数（入账快照）
	GroupBuyID    int64     `gorm:"default:0" json:"groupBuyId"`   // 所属拼团，0=普通单
	Remark        string    `gorm:"size:255" json:"remark"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (Order) TableName() string { return "orders" }

// Payment 支付流水。transaction_id 可空 + 唯一键（MySQL 唯一索引允许多 NULL）：
// 微信回调会重试且可能并发到达，回调处理以「条件 UPDATE WHERE status='created'」
// 的影响行数为幂等闸门，transaction_id 唯一键做 DB 级兜底防重放。
type Payment struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID       int64     `gorm:"index:idx_payment_order" json:"orderId"`
	TransactionID *string   `gorm:"column:transaction_id;size:64;uniqueIndex" json:"transactionId"`
	Amount        int64     `json:"amount"`
	Status        string    `gorm:"size:16;default:created" json:"status"` // created/success/failed
	RawNotify     string    `json:"-"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (Payment) TableName() string { return "payment" }

// 退款单状态
const (
	RefundStatusPending  = "pending"
	RefundStatusApproved = "approved"
	RefundStatusRejected = "rejected"
	RefundStatusRefunded = "refunded"
	RefundStatusFailed   = "failed"
)

// Refund 退款单。审批时快照（unit_price/reclaimed_sessions/calc_type）
// 在事务内重读重算后落库，保证金额与账实一致、可对账（P0-3）。
type Refund struct {
	ID                int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	RefundNo          string    `gorm:"column:refund_no;size:32;uniqueIndex" json:"refundNo"`
	OrderID           int64     `gorm:"index:idx_refund_order" json:"orderId"`
	UserID            int64     `json:"userId"`
	Amount            int64     `json:"amount"`
	Reason            string    `gorm:"size:255" json:"reason"`
	Status            string    `gorm:"size:16;default:pending" json:"status"`
	RejectReason      string    `gorm:"size:255" json:"rejectReason"`
	AdminID           int64     `gorm:"default:0" json:"adminId"`
	UnitPrice         int64     `gorm:"default:0" json:"unitPrice"`             // 审批时快照：单课时折算价(分)
	ReclaimedSessions int       `gorm:"default:0" json:"reclaimedSessions"`    // 审批时快照：回收课时数
	CalcType          string    `gorm:"size:16;default:''" json:"calcType"`    // full/remainder/full_no_mp
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

func (Refund) TableName() string { return "refund" }
