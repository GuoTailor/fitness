package model

import "time"

// 课包分类
const (
	PkgCategoryPrivate = "private" // 私教 1v1
	PkgCategoryGroup   = "group"   // 团课卡
	PkgCategoryTrial   = "trial"   // 体验课
)

// CoursePackage 课包
type CoursePackage struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID       int64     `gorm:"column:owner_id;default:1;index:idx_pkg_owner" json:"-"`
	Title         string    `gorm:"size:128" json:"title"`
	CoverURL      string    `gorm:"size:512" json:"coverUrl"`
	IntroVideoURL string    `gorm:"size:512" json:"introVideoUrl"`
	Category      string    `gorm:"size:16;default:private;index:idx_pkg_owner" json:"category"`
	Tags          string    `gorm:"size:255" json:"tags"` // 逗号分隔：减脂,增肌
	Difficulty    int8      `gorm:"default:1" json:"difficulty"`
	Outline       string    `gorm:"type:text" json:"outline"`
	Suitable      string    `gorm:"type:text" json:"suitable"`
	Status        int8      `gorm:"default:0;index:idx_pkg_owner" json:"status"` // 1 上架
	SoldCount     int       `json:"soldCount"`
	Sort          int       `json:"sort"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (CoursePackage) TableName() string { return "course_package" }

// PackageSKU 课包规格（10 次卡 / 20 次卡…）
type PackageSKU struct {
	ID            int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	PackageID     int64  `gorm:"index:idx_sku_pkg" json:"packageId"`
	Name          string `gorm:"size:64" json:"name"`
	Sessions      int    `json:"sessions"`
	ValidDays     int    `gorm:"default:365" json:"validDays"`
	Price         int64  `json:"price"`         // 分
	OriginalPrice int64  `json:"originalPrice"` // 分
	NewcomerPrice *int64 `json:"newcomerPrice"` // P1 预留
	// 限时活动价（P1）：三列同时设置且当前时间落在 [start, end) 内才生效
	PromoPrice *int64     `json:"promoPrice"`
	PromoStart *time.Time `json:"promoStart"`
	PromoEnd   *time.Time `json:"promoEnd"`
	// 拼团（P2）：GroupPrice>0 且 GroupCapacity 2~5 时开启；拼团价不与券/新人价/活动价叠加
	GroupPrice    int64     `gorm:"default:0" json:"groupPrice"`
	GroupCapacity int       `gorm:"default:0" json:"groupCapacity"`
	Status        int8      `gorm:"default:1;index:idx_sku_pkg" json:"status"`
	Sort          int       `json:"sort"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (PackageSKU) TableName() string { return "package_sku" }

// CoachPackage 教练在售课包关联
type CoachPackage struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CoachID   int64     `gorm:"uniqueIndex:uk_coach_pkg" json:"coachId"`
	PackageID int64     `gorm:"uniqueIndex:uk_coach_pkg" json:"packageId"`
	CreatedAt time.Time `json:"createdAt"`
}

func (CoachPackage) TableName() string { return "coach_package" }

// Banner 首页运营位
type Banner struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID   int64     `gorm:"column:owner_id;default:1;index:idx_banner_owner" json:"-"`
	ImageURL  string    `gorm:"size:512" json:"imageUrl"`
	LinkType  string    `gorm:"size:16;default:none" json:"linkType"` // none/package/coach
	LinkValue string    `gorm:"size:128" json:"linkValue"`
	Sort      int       `json:"sort"`
	Status    int8      `gorm:"default:1;index:idx_banner_owner" json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Banner) TableName() string { return "banner" }
