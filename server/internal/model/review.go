package model

import "time"

// 核销状态
const (
	VerifyStatusPending   = "pending"
	VerifyStatusVerified  = "verified"
	VerifyStatusCancelled = "cancelled"
	VerifyStatusExpired   = "expired"
)

// Verification 核销记录（学员出示码，教练/前台扫码）
type Verification struct {
	ID              int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	BizType         string     `gorm:"size:16" json:"bizType"` // private/group
	BookingID       int64      `gorm:"default:0;index" json:"bookingId"`
	EnrollmentID    int64      `gorm:"default:0" json:"enrollmentId"`
	UserID          int64      `gorm:"index:idx_verify_user" json:"userId"`
	CoachID         int64      `gorm:"index:idx_verify_coach" json:"coachId"`
	MemberPackageID int64      `json:"memberPackageId"`
	Sessions        int        `gorm:"default:1" json:"sessions"`
	Code            string     `gorm:"size:32;uniqueIndex" json:"code"`
	Status          string     `gorm:"size:16;default:pending;index:idx_verify_user;index:idx_verify_coach" json:"status"`
	VerifiedAt      *time.Time `json:"verifiedAt"`
	VerifiedBy      string     `gorm:"size:16" json:"verifiedBy"` // coach/admin
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func (Verification) TableName() string { return "verification" }

// TrainingRecord 训练记录（核销自动生成）
type TrainingRecord struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID          int64     `gorm:"index:idx_tr_user" json:"userId"`
	CoachID         int64     `json:"coachId"`
	PackageID       int64     `json:"packageId"`
	RecordDate      string    `gorm:"column:record_date;type:date;index:idx_tr_user" json:"recordDate"` // YYYY-MM-DD
	Project         string    `gorm:"size:64" json:"project"`
	DurationMinutes int       `json:"durationMinutes"`
	Source          string    `gorm:"size:16;default:private" json:"source"` // private/group
	Remark          string    `gorm:"size:255" json:"remark"`
	CreatedAt       time.Time `json:"createdAt"`
}

func (TrainingRecord) TableName() string { return "training_record" }

// 评价维度与状态
const (
	ReviewTargetPackage  = "package"
	ReviewTargetCoach    = "coach"
	ReviewStatusPending  = "pending"
	ReviewStatusApproved = "approved"
	ReviewStatusHidden   = "hidden"
)

// Review 评价（双维度：课包评价 + 教练评价，各一条记录）。
// uk_review(verification_id, target_type) 保证同一核销每维度只评一次。
type Review struct {
	ID             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID        int64      `gorm:"column:owner_id;default:1" json:"-"`
	UserID         int64      `json:"userId"`
	TargetType     string     `gorm:"size:16;uniqueIndex:uk_review;index:idx_review_pkg;index:idx_review_coach" json:"targetType"`
	PackageID      int64      `gorm:"index:idx_review_pkg" json:"packageId"`
	CoachID        int64      `gorm:"index:idx_review_coach" json:"coachId"`
	VerificationID int64      `gorm:"uniqueIndex:uk_review" json:"verificationId"` // 防刷单：完成核销才可评价
	Rating         int8       `json:"rating"`
	Content        string     `gorm:"type:text" json:"content"`
	Images         string     `gorm:"type:text" json:"images"` // JSON 数组
	Status         string     `gorm:"size:16;default:pending;index:idx_review_pkg;index:idx_review_coach;index:idx_review_status" json:"status"`
	Featured       int8       `json:"featured"`
	CoachReply     string     `gorm:"type:text" json:"coachReply"`
	CoachReplyAt   *time.Time `json:"coachReplyAt"`
	CreatedAt      time.Time  `gorm:"index:idx_review_status" json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

func (Review) TableName() string { return "review" }
