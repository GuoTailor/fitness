package model

import "time"

// 拼团状态
const (
	GroupBuyEnrolling = 1 // 报名中
	GroupBuySuccess   = 2 // 已成团
	GroupBuyFailed    = 3 // 未成团（到期自动退款）
)

// GroupBuy 拼团（price/capacity 为开团时的 SKU 快照）
type GroupBuy struct {
	ID           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID      int64      `gorm:"column:owner_id;default:1" json:"-"`
	SkuID        int64      `gorm:"index:idx_gb_sku_status" json:"skuId"`
	PackageID    int64      `json:"packageId"`
	Capacity     int        `json:"capacity"`
	Price        int64      `json:"price"`
	Status       int8       `gorm:"default:1;index:idx_gb_sku_status" json:"status"`
	LeaderUserID int64      `json:"leaderUserId"`
	ExpireAt     time.Time  `gorm:"index:idx_gb_expire" json:"expireAt"`
	SuccessAt    *time.Time `json:"successAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

func (GroupBuy) TableName() string { return "group_buy" }

// GroupBuyMember 参团成员（团+人唯一：一人一团单次；订单随行）
type GroupBuyMember struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupBuyID int64     `gorm:"uniqueIndex:uk_gbm_group_user" json:"groupBuyId"`
	UserID     int64     `gorm:"uniqueIndex:uk_gbm_group_user;index:idx_gbm_user" json:"userId"`
	OrderID    int64     `json:"orderId"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (GroupBuyMember) TableName() string { return "group_buy_member" }
