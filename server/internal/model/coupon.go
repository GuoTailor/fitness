package model

import "time"

// 券类型
const (
	CouponTypeReduce   = "reduce"   // 满减券：amount=减免分，满 threshold 可用
	CouponTypeDiscount = "discount" // 折扣券：amount=折扣率万分比（8800=88折）
	CouponTypeGift     = "gift"     // 赠课券：amount=赠送课时数，用券下单后随入账赠送（不降价）
)

// 券实例状态
const (
	CouponUsable  = "usable"
	CouponUsed    = "used"
	CouponExpired = "expired"
)

// CouponTemplate 优惠券模板（后台创建；发放渠道：领券中心 / 后台定向 / 新人礼包自动发放）
type CouponTemplate struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID      int64     `gorm:"column:owner_id;default:1" json:"-"`
	Name         string    `gorm:"size:64" json:"name"`
	Type         string    `gorm:"size:16;default:reduce" json:"type"`
	Threshold    int64     `json:"threshold"`                // 满 X 分可用，0=无门槛
	Amount       int64     `json:"amount"`                   // reduce=减免分 / discount=万分比
	PackageID    int64     `gorm:"default:0" json:"packageId"` // 限定课包，0=全场通用
	TotalCount   int       `gorm:"default:0" json:"totalCount"` // 发放总量，0=不限
	IssuedCount  int       `gorm:"default:0" json:"issuedCount"`
	PerUserLimit int       `gorm:"default:1" json:"perUserLimit"`
	ValidDays    int       `gorm:"default:30" json:"validDays"` // 领取后 N 天过期
	Status       int8      `gorm:"default:1" json:"status"`
	Remark       string    `gorm:"size:255" json:"remark"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (CouponTemplate) TableName() string { return "coupon_template" }

// UserCoupon 用户持券实例
type UserCoupon struct {
	ID         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     int64      `gorm:"index:idx_uc_user" json:"userId"`
	TemplateID int64      `gorm:"index:idx_uc_tpl" json:"templateId"`
	Status     string     `gorm:"size:16;default:usable;index:idx_uc_user" json:"status"`
	OrderID    int64      `gorm:"default:0" json:"orderId"`
	ExpireAt   *time.Time `json:"expireAt"`
	UsedAt     *time.Time `json:"usedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

func (UserCoupon) TableName() string { return "user_coupon" }
