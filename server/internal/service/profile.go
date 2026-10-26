package service

import (
	"context"
	"net/url"
	"strings"

	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
)

// UpdateUserProfile 个人资料编辑（昵称/头像）。头像须来自本站上传（/uploads/images/ 路径），空=保持不变。
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
		// P1-19：解析 URL 后校验路径前缀，防 `https://evil.com/?x=/uploads/` 这类 Contains 绕过；
		// 允许站内相对路径与 http(s) 绝对地址，拒绝 protocol-relative 等异常形态
		u, err := url.Parse(avatarURL)
		if err != nil || !strings.HasPrefix(u.Path, "/uploads/images/") ||
			(u.Host != "" && u.Scheme != "http" && u.Scheme != "https") {
			return nil, errs.BadParam("头像必须来自站内上传")
		}
		if len(avatarURL) > 512 {
			return nil, errs.BadParam("头像链接异常")
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
