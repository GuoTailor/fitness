package service

import (
	"context"
	"log"

	"gorm.io/gorm"

	"fitness/internal/model"
	"fitness/internal/wechat"
)

// NotifyService 订阅消息。发送前置写 notify_log，uk_notify(user_id, scene, biz_id)
// 冲突即视为已发过直接返回——定时任务重复扫描、业务重试都不会重复打扰用户。
// mock 模式只落库不调微信；真实模式要求 notify.templates 配置了对应场景模板 ID。
type NotifyService struct {
	db        *gorm.DB
	mp        wechat.MiniProgram
	templates map[string]string
	mock      bool
}

func NewNotifyService(db *gorm.DB, mp wechat.MiniProgram, mock bool, templates map[string]string) *NotifyService {
	return &NotifyService{db: db, mp: mp, templates: templates, mock: mock}
}

// Send 发送订阅消息（幂等：同 (user, scene, biz) 只发一次）。
// sent=false 表示此前已发过（幂等跳过）；错误仅记录不阻断主流程（调用方用 tryNotify 包装）。
func (s *NotifyService) Send(ctx context.Context, userID int64, scene string, bizID int64, data map[string]string) (bool, error) {
	if s == nil || s.db == nil || userID <= 0 {
		return false, nil
	}
	tplID := s.templates[scene]
	if !s.mock && tplID == "" {
		return false, nil // 真实模式下未配置模板的场景直接跳过
	}
	row := model.NotifyLog{UserID: userID, Scene: scene, BizID: bizID, Status: "sent"}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		if isDuplicateKey(err) {
			return false, nil // 已发过
		}
		return false, err
	}
	if s.mock {
		return true, nil
	}
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		s.markFail(ctx, row.ID, err.Error())
		return false, err
	}
	if err := s.mp.SendSubscribe(ctx, user.OpenID, tplID, data); err != nil {
		s.markFail(ctx, row.ID, err.Error())
		return false, err
	}
	return true, nil
}

// markFail 发送失败：删除记录释放 (user, scene, biz) 键位，让定时任务下次扫描重发；
// 否则失败的那次恰好是唯一触达机会（uk 占位后永远不再发送）。
func (s *NotifyService) markFail(ctx context.Context, id int64, msg string) {
	if len(msg) > 250 {
		msg = msg[:250]
	}
	if err := s.db.WithContext(ctx).
		Where("id = ? AND status = ?", id, "sent").
		Updates(map[string]interface{}{"status": "fail", "err_msg": msg}).Error; err != nil {
		return
	}
	s.db.WithContext(ctx).Where("id = ? AND status = ?", id, "fail").Delete(&model.NotifyLog{})
	log.Printf("[notify] 发送失败已记录并释放重发键位: %s", msg)
}

// tryNotify 业务内发送：失败只打日志，不影响主流程返回值。
func tryNotify(ctx context.Context, n *NotifyService, userID int64, scene string, bizID int64, data map[string]string) {
	if n == nil {
		return
	}
	if _, err := n.Send(ctx, userID, scene, bizID, data); err != nil {
		log.Printf("[notify] %s biz=%d 发送失败: %v", scene, bizID, err)
	}
}
