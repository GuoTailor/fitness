package model

import "time"

// 问答内容状态（先发后审：默认 visible，后台隐藏违规内容）
const (
	QaStatusVisible = "visible"
	QaStatusHidden  = "hidden"
)

// QaQuestion 课包问答提问（购前疑虑，登录即可提问，可选匿名展示）
type QaQuestion struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID   int64     `gorm:"column:owner_id;default:1" json:"-"`
	PackageID int64     `gorm:"index:idx_q_pkg,priority:1" json:"packageId"`
	UserID    int64     `gorm:"index:idx_q_user" json:"userId"`
	Content   string    `gorm:"size:500" json:"content"`
	Anonymous int8      `json:"anonymous"`
	Status    string    `gorm:"size:16;default:visible;index:idx_q_pkg,priority:2" json:"status"`
	Pinned    int8      `json:"pinned"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (QaQuestion) TableName() string { return "qa_question" }

// QaAnswer 问答回复（教练带认证标识、排序优先；已购学员也可回复）
type QaAnswer struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID    int64     `gorm:"column:owner_id;default:1" json:"-"`
	QuestionID int64     `gorm:"index:idx_a_q,priority:1" json:"questionId"`
	UserID     int64     `gorm:"index:idx_a_user" json:"userId"`
	CoachID    int64     `gorm:"default:0" json:"coachId"`
	IsCoach    int8      `json:"isCoach"`
	Content    string    `gorm:"size:1000" json:"content"`
	Status     string    `gorm:"size:16;default:visible;index:idx_a_q,priority:2" json:"status"`
	Pinned     int8      `json:"pinned"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (QaAnswer) TableName() string { return "qa_answer" }
