package model

import "time"

// 课时账户状态
const (
	MPStatusInactive  = "inactive"  // 未开卡（购入后首次扣次前）
	MPStatusActive    = "active"    // 生效中
	MPStatusExhausted = "exhausted" // 已用完
	MPStatusExpired   = "expired"   // 已过期
	MPStatusRefunded  = "refunded"  // 已退款回收
)

// MemberPackage 学员课包（课时账户）
type MemberPackage struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID       int64      `gorm:"column:owner_id;default:1" json:"-"`
	UserID        int64      `gorm:"index:idx_mp_user" json:"userId"`
	PackageID     int64      `json:"packageId"`
	SkuID         int64      `json:"skuId"`
	OrderID       int64      `gorm:"default:0" json:"orderId"`
	PackageTitle  string     `gorm:"size:128" json:"packageTitle"`
	TotalSessions int        `json:"totalSessions"`
	UsedSessions  int        `json:"usedSessions"`                  // 冗余余额，以 package_transaction 流水为准
	ValidDays     int        `gorm:"default:365" json:"validDays"`  // 购买时快照
	UnitPrice     int64      `gorm:"default:0" json:"unitPrice"`    // 购买时快照：单课时折算价（分）
	GiftSessions  int        `gorm:"default:0" json:"giftSessions"` // 赠课券赠送课时（退款折算不计入可退）
	ExpireDate    *time.Time `gorm:"type:date" json:"expireDate"`
	Status        string     `gorm:"size:16;default:inactive;index:idx_mp_user;index:idx_mp_expire" json:"status"`
	ActivatedAt   *time.Time `json:"activatedAt"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func (MemberPackage) TableName() string { return "member_package" }

// RemainingSessions 剩余课时
func (m *MemberPackage) RemainingSessions() int {
	return m.TotalSessions - m.UsedSessions
}

// 课时流水类型
const (
	TxnTypePurchase      = "purchase"       // 购买入账
	TxnTypeDeduct        = "deduct"         // 核销扣除
	TxnTypeBookingRefund = "booking_cancel" // 预约取消返还（penalty 之外的）
	TxnTypePenalty       = "penalty"        // 开课前取消扣次（不返还）
	TxnTypeRefundReclaim = "refund_reclaim" // 退款回收
	TxnTypeRefundRestore = "refund_restore" // 退款失败恢复课时（转人工）
	TxnTypeManualAdjust  = "manual_adjust"  // 后台手动调整（赠课/补课）
	TxnTypeGift          = "gift"           // 赠课券赠送（随订单入账）
)

// PackageTransaction 课时流水——所有课时变动的唯一入口。
// idempotency_key 唯一约束保证扣次/返还幂等（防重复核销、防回调重放）。
type PackageTransaction struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	MemberPackageID int64     `gorm:"index:idx_txn_mp" json:"memberPackageId"`
	UserID          int64     `gorm:"index:idx_txn_user" json:"userId"`
	SessionsChange  int       `gorm:"column:sessions_change" json:"sessionsChange"`
	Type            string    `gorm:"size:24" json:"type"`
	IdempotencyKey  string    `gorm:"column:idempotency_key;size:64;uniqueIndex" json:"idempotencyKey"`
	BizType         string    `gorm:"column:biz_type;size:16" json:"bizType"`
	BizID           int64     `gorm:"column:biz_id;default:0" json:"bizId"`
	Remark          string    `gorm:"size:255" json:"remark"`
	CreatedAt       time.Time `gorm:"index:idx_txn_user" json:"createdAt"`
}

func (PackageTransaction) TableName() string { return "package_transaction" }
