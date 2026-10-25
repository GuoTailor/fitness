package service

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
)

// ReferralService 老带新裂变：落地页绑定邀请关系，被邀请人首次实付后双方发券。
// 合规约束：奖励只发券（不结算现金、不搞多级分销）；绑定 30 天内下单才有效。
type ReferralService struct {
	db      *gorm.DB
	coupons *CouponService // 可为 nil（单测不接发券）
	cfg     *ConfigService
}

func NewReferralService(db *gorm.DB, coupons *CouponService, cfg *ConfigService) *ReferralService {
	return &ReferralService{db: db, coupons: coupons, cfg: cfg}
}

// referrerRewardWindow 邀请奖励有效期：绑定后 30 天内首次实付才发奖励
const referrerRewardWindow = 30 * 24 * time.Hour

// Bind 被邀请人绑定邀请关系（落地后调用；服务端幂等，重复绑定明确拒绝）
func (s *ReferralService) Bind(ctx context.Context, inviteeID, inviterID int64) error {
	if inviterID <= 0 {
		return errs.BadParam("邀请码无效")
	}
	if inviterID == inviteeID {
		return errs.Biz("不能邀请自己")
	}
	var inviter model.User
	if err := s.db.WithContext(ctx).
		Where("id = ? AND status = 1", inviterID).First(&inviter).Error; err != nil {
		return errs.NotFound("邀请人不存在")
	}
	var exist model.ReferralRecord
	if err := s.db.WithContext(ctx).Where("invitee_id = ?", inviteeID).First(&exist).Error; err == nil {
		return errs.Biz("你已绑定过邀请人")
	}
	// 老客不符合邀请条件（防自邀刷券）：名下已有过实付订单
	var paidOrders int64
	if err := s.db.WithContext(ctx).Model(&model.Order{}).
		Where("user_id = ? AND status IN ?", inviteeID,
			[]string{model.OrderStatusPaid, model.OrderStatusRefund, model.OrderStatusRefunded}).
		Count(&paidOrders).Error; err != nil {
		return err
	}
	if paidOrders > 0 {
		return errs.Biz("已有购课记录，不符合邀请条件")
	}
	rec := model.ReferralRecord{InviterID: inviterID, InviteeID: inviteeID, Status: model.ReferralBound}
	if err := s.db.WithContext(ctx).Create(&rec).Error; err != nil {
		// uk_ref_invitee 兜底并发重复绑定
		var exist2 model.ReferralRecord
		if s.db.WithContext(ctx).Where("invitee_id = ?", inviteeID).First(&exist2).Error == nil {
			return errs.Biz("你已绑定过邀请人")
		}
		return err
	}
	return nil
}

// RewardOnPaidOrder 被邀请人支付成功后尝试发奖励（幂等：条件 UPDATE 1→2 抢闸门，双单只奖一次）。
// 由 HandlePaySuccess 事务外调用，失败仅记日志不影响支付主流程。
func (s *ReferralService) RewardOnPaidOrder(ctx context.Context, inviteeID int64) {
	var rec model.ReferralRecord
	if err := s.db.WithContext(ctx).
		Where("invitee_id = ? AND status = ?", inviteeID, model.ReferralBound).
		First(&rec).Error; err != nil {
		return // 未绑定或已奖励
	}
	if time.Since(rec.CreatedAt) > referrerRewardWindow {
		return // 绑定超 30 天未下单，奖励失效
	}
	// 幂等闸门：并发支付只有一单能抢到
	res := s.db.WithContext(ctx).Model(&model.ReferralRecord{}).
		Where("id = ? AND status = ?", rec.ID, model.ReferralBound).
		Updates(map[string]interface{}{"status": model.ReferralRewared, "rewarded_at": time.Now()})
	if res.Error != nil || res.RowsAffected == 0 {
		return
	}
	if s.coupons == nil {
		return
	}
	// 模板未配置 = 奖励关闭，绑定关系仍然生效
	grant := func(who string, userID int64, key string) {
		val := s.cfg.Get(ctx, key, "")
		if val == "" {
			return
		}
		tplID, err := strconv.ParseInt(val, 10, 64)
		if err != nil || tplID <= 0 {
			log.Printf("[referral] 配置 %s=%q 非法，跳过", key, val)
			return
		}
		if _, err := s.coupons.Grant(ctx, userID, tplID); err != nil {
			log.Printf("[referral] %s 奖励券发放失败 user=%d tpl=%d: %v", who, userID, tplID, err)
		}
	}
	grant("邀请人", rec.InviterID, model.CfgReferralInviterTemplate)
	grant("被邀请人", inviteeID, model.CfgReferralInviteeTemplate)
}

// Mine 我的邀请状态（邀请码即用户 ID）
func (s *ReferralService) Mine(ctx context.Context, userID int64) (map[string]interface{}, error) {
	out := map[string]interface{}{
		"inviteCode": userID, "bound": false, "rewarded": false,
	}
	var rec model.ReferralRecord
	if err := s.db.WithContext(ctx).Where("invitee_id = ?", userID).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return out, nil
		}
		return nil, err
	}
	out["bound"] = true
	out["rewarded"] = rec.Status == model.ReferralRewared
	out["invitedAt"] = rec.CreatedAt
	if rec.RewardedAt != nil {
		out["rewardedAt"] = rec.RewardedAt
	}
	var inviter model.User
	if err := s.db.WithContext(ctx).Select("id, nickname, avatar_url").
		First(&inviter, rec.InviterID).Error; err == nil {
		out["inviter"] = map[string]interface{}{
			"id": inviter.ID, "nickname": inviter.Nickname, "avatarUrl": inviter.AvatarURL,
		}
	}
	return out, nil
}

// AdminList 后台邀请记录（带双方昵称）
func (s *ReferralService) AdminList(ctx context.Context, page, pageSize int) ([]map[string]interface{}, int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Model(&model.ReferralRecord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var recs []model.ReferralRecord
	if err := s.db.WithContext(ctx).
		Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&recs).Error; err != nil {
		return nil, 0, err
	}
	ids := map[int64]bool{}
	for _, r := range recs {
		ids[r.InviterID] = true
		ids[r.InviteeID] = true
	}
	names := map[int64]string{}
	if len(ids) > 0 {
		var users []model.User
		if err := s.db.WithContext(ctx).Select("id, nickname, phone").
			Where("id IN ?", keysOf(ids)).Find(&users).Error; err == nil {
			for _, u := range users {
				names[u.ID] = u.Nickname
			}
		}
	}
	out := make([]map[string]interface{}, 0, len(recs))
	for _, r := range recs {
		out = append(out, map[string]interface{}{
			"id": r.ID, "inviterId": r.InviterID, "inviterName": nameOf(names, r.InviterID),
			"inviteeId": r.InviteeID, "inviteeName": nameOf(names, r.InviteeID),
			"status": r.Status, "createdAt": r.CreatedAt, "rewardedAt": r.RewardedAt,
		})
	}
	return out, total, nil
}

func keysOf(m map[int64]bool) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func nameOf(names map[int64]string, id int64) string {
	if n, ok := names[id]; ok && n != "" {
		return n
	}
	return fmt.Sprintf("用户%d", id)
}
