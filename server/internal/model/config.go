package model

import "time"

// 运营配置键（后台可改，值均存字符串）
const (
	CfgBookingCancelPenaltyHours = "booking.cancel_penalty_hours"
	CfgGroupQuitDeadlineHours    = "group_class.quit_deadline_hours"
	CfgRefundFullDays            = "refund.full_days"
	CfgMemberWarnSessions        = "member.warn_sessions"
	CfgMemberWarnExpireDays      = "member.warn_expire_days"
	CfgReviewInviteAfterDays     = "review.invite_after_days"
	CfgReviewAutoApprove         = "review.auto_approve"                 // 1=评价免审直接通过
	CfgNewcomerGiftTemplates     = "marketing.newcomer_gift_templates"   // 新人礼包券模板 ID CSV，空=不发
	CfgReviewRewardTemplate      = "marketing.review_reward_template"    // 评价奖励券模板 ID，空=不发
	CfgRenewalReminderTemplate   = "marketing.renewal_reminder_template" // 续费提醒券模板 ID，空=不发
	CfgReferralInviterTemplate   = "marketing.referral_inviter_template" // 老带新-邀请人奖励券模板 ID，空=不发
	CfgReferralInviteeTemplate   = "marketing.referral_invitee_template" // 老带新-被邀请人奖励券模板 ID，空=不发
	CfgCheckinTarget             = "marketing.checkin_target"            // 打卡挑战：每月上课达标次数，空=关闭
	CfgCheckinRewardTemplate     = "marketing.checkin_reward_template"   // 打卡挑战奖励券模板 ID，空=不发
	CfgGroupExpireHours          = "marketing.group_expire_hours"        // 拼团有效期（小时），默认 48
	CfgPrivacyContent            = "misc.privacy_content"                // 隐私政策正文
	CfgAgreementContent          = "misc.agreement_content"              // 用户协议正文
)

// AppConfig 运营配置 KV
type AppConfig struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Key       string    `gorm:"column:cfg_key;size:64;uniqueIndex" json:"key"`
	Value     string    `gorm:"column:cfg_value;size:512" json:"value"`
	Remark    string    `gorm:"size:255" json:"remark"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (AppConfig) TableName() string { return "app_config" }

// 订阅消息场景
const (
	NotifySceneBookingConfirmed = "booking_confirmed"  // 教练确认 → 通知学员
	NotifySceneBookingNew       = "booking_new"        // 新预约 → 通知教练
	NotifySceneReviewInvite     = "review_invite"      // 核销次日评价邀请
	NotifySceneSessionsWarning  = "sessions_warning"   // 课时不足
	NotifySceneExpireWarning    = "expire_warning"     // 快到期
	NotifySceneWaitlistPromoted = "waitlist_promoted"  // 团课候补递补成功
	NotifySceneFeedbackNew      = "feedback_new"       // 教练课后反馈
	NotifySceneQANew            = "qa_new"             // 课程新提问 → 通知教练
	NotifySceneQAReply          = "qa_reply"           // 问答新回复 → 通知提问者
	NotifySceneGroupBuySuccess  = "groupbuy_success"   // 拼团成团
	NotifySceneGroupBuyFail     = "groupbuy_fail"      // 拼团未成团自动退款
	NotifySceneCoachApplyResult = "coach_apply_result" // 教练入驻审核结果
)

// NotifyLog 订阅消息发送记录（uk 防重发）
type NotifyLog struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"uniqueIndex:uk_notify" json:"userId"`
	Scene     string    `gorm:"size:32;uniqueIndex:uk_notify" json:"scene"`
	BizID     int64     `gorm:"uniqueIndex:uk_notify" json:"bizId"`
	Status    string    `gorm:"size:8;default:sent" json:"status"`
	ErrMsg    string    `gorm:"size:255" json:"errMsg"`
	CreatedAt time.Time `json:"createdAt"`
}

func (NotifyLog) TableName() string { return "notify_log" }
