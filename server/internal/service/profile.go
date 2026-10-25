package service

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
)

// UpdateUserProfile 个人资料编辑（昵称/头像）。头像须来自本站上传（/uploads/ 路径），空=保持不变。
func UpdateUserProfile(ctx context.Context, db *gorm.DB, userID int64, nickname, avatarURL string) (*model.User, error) {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" {
		return nil, errs.BadParam("昵称不能为空")
	}
	if len([]rune(nickname)) > 64 {
		return nil, errs.BadParam("昵称最多 64 字")
	}
	avatarURL = strings.TrimSpace(avatarURL)
	if avatarURL != "" {
		if len(avatarURL) > 512 || !strings.Contains(avatarURL, "/uploads/") {
			return nil, errs.BadParam("头像必须来自站内上传")
		}
	}
	updates := map[string]interface{}{"nickname": nickname}
	if avatarURL != "" {
		updates["avatar_url"] = avatarURL
	}
	if err := db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).Updates(updates).Error; err != nil {
		return nil, errs.Server(err, "保存资料失败")
	}
	var u model.User
	if err := db.WithContext(ctx).First(&u, userID).Error; err != nil {
		return nil, errs.NotFound("用户不存在")
	}
	return &u, nil
}
