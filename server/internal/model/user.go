package model

import "time"

// User 学员（微信登录）
type User struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OpenID    string    `gorm:"column:openid;size:64;uniqueIndex" json:"-"`
	UnionID   string    `gorm:"column:unionid;size:64" json:"-"`
	Nickname  string    `gorm:"size:64" json:"nickname"`
	AvatarURL string    `gorm:"size:512" json:"avatarUrl"`
	Phone     string    `gorm:"size:20;index" json:"phone"`
	Status    int8      `gorm:"default:1" json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (User) TableName() string { return "user" }

// Coach 教练（coach.user_id 绑定登录账号实现小程序双角色）
type Coach struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID      int64     `gorm:"column:owner_id;default:1;index:idx_coach_owner" json:"-"`
	UserID       int64     `gorm:"column:user_id;index;default:0" json:"userId"`
	Name         string    `gorm:"size:64" json:"name"`
	AvatarURL    string    `gorm:"size:512" json:"avatarUrl"`
	Gender       int8      `json:"gender"`
	Years        int       `json:"years"`
	Specialties  string    `gorm:"size:255" json:"specialties"`
	CertImages   string    `json:"certImages"`
	CertVerified int8      `json:"certVerified"`
	Intro        string    `gorm:"type:text" json:"intro"`
	IntroVideo   string    `gorm:"column:intro_video_url;size:512" json:"introVideoUrl"`
	RatingAvg    float64   `gorm:"column:rating_avg;type:decimal(3,2);default:0" json:"ratingAvg"`
	RatingCount  int       `json:"ratingCount"`
	Status       int8      `gorm:"index:idx_coach_owner" json:"status"`
	Sort         int       `json:"sort"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (Coach) TableName() string { return "coach" }

const (
	AdminRoleBoss         = "boss"
	AdminRoleReceptionist = "receptionist"
)

// AdminUser 管理后台账号（老板/前台）
type AdminUser struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex" json:"username"`
	PasswordHash string    `gorm:"size:100" json:"-"`
	Name         string    `gorm:"size:64" json:"name"`
	Role         string    `gorm:"size:16;default:receptionist" json:"role"`
	Status       int8      `gorm:"default:1" json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (AdminUser) TableName() string { return "admin_user" }
