package model

import "time"

// 拼团状态
const (
	GroupBuyEnrolling = 1 // 报名中
	GroupBuySuccess   = 2 // 已成团
	GroupBuyFailed    = 3 // 未成团（到期自动退款）
)

// GroupBuy 拼团（price/capacity 为开团时的取值快照：来自限时活动或 SKU 常开配置）
type GroupBuy struct {
	ID           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID      int64      `gorm:"column:owner_id;default:1" json:"-"`
	SkuID        int64      `gorm:"index:idx_gb_sku_status" json:"skuId"`
	PackageID    int64      `json:"packageId"`
	Capacity     int        `json:"capacity"`
	Price        int64      `json:"price"`
	Status       int8       `gorm:"default:1;index:idx_gb_sku_status" json:"status"`
	LeaderUserID int64      `json:"leaderUserId"`
	ActivityID   int64      `gorm:"default:0" json:"activityId"` // 0=SKU 常开配置
	ExpireAt     time.Time  `gorm:"index:idx_gb_expire" json:"expireAt"`
	SuccessAt    *time.Time `json:"successAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

func (GroupBuy) TableName() string { return "group_buy" }

// GroupBuyActivity 拼团限时活动（P3）：生效期内开团用活动价/人数/时长，
// 到点自动失效回落 SKU 常开配置；名额按开团数计，opened_count 事务内原子扣减。
type GroupBuyActivity struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID       int64     `gorm:"column:owner_id;default:1" json:"-"`
	Title         string    `gorm:"size:64" json:"title"`
	SkuID         int64     `gorm:"index:idx_gba_sku" json:"skuId"`
	PackageID     int64     `json:"packageId"`
	GroupPrice    int64     `json:"groupPrice"`
	GroupCapacity int       `json:"groupCapacity"`
	StartAt       time.Time `json:"startAt"`
	EndAt         time.Time `json:"endAt"`
	MaxGroups     int       `gorm:"default:0" json:"maxGroups"`    // 0=不限
	OpenedCount   int       `gorm:"default:0" json:"openedCount"`  // 已开团数
	ExpireHours   int       `gorm:"default:0" json:"expireHours"`  // 0=用全局配置
	Status        int8      `gorm:"default:1;index:idx_gba_status" json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (GroupBuyActivity) TableName() string { return "group_buy_activity" }

// GroupBuyMember 参团成员（团+人唯一：一人一团单次；订单随行）
type GroupBuyMember struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupBuyID int64     `gorm:"uniqueIndex:uk_gbm_group_user" json:"groupBuyId"`
	UserID     int64     `gorm:"uniqueIndex:uk_gbm_group_user;index:idx_gbm_user" json:"userId"`
	OrderID    int64     `json:"orderId"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (GroupBuyMember) TableName() string { return "group_buy_member" }
