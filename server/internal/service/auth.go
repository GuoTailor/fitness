// Package service 业务逻辑层。
package service

import (
	"context"
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"fitness/internal/auth"
	"fitness/internal/errs"
	"fitness/internal/middleware"
	"fitness/internal/model"
	"fitness/internal/wechat"
)

type AuthService struct {
	db      *gorm.DB
	tokens  *auth.Manager
	mp      wechat.MiniProgram
	lockout *middleware.AdminLockout // 可为 nil（单测不启用）
}

func NewAuthService(db *gorm.DB, tokens *auth.Manager, mp wechat.MiniProgram) *AuthService {
	return &AuthService{db: db, tokens: tokens, mp: mp}
}

// WithLockout 启用后台登录失败锁定
func (s *AuthService) WithLockout(l *middleware.AdminLockout) *AuthService {
	s.lockout = l
	return s
}

// LoginResult 登录返回：token + 用户信息 + 是否教练（双角色）
type LoginResult struct {
	Token    string      `json:"token"`
	User     *model.User `json:"user"`
	IsCoach  bool        `json:"isCoach"`
	CoachID  int64       `json:"coachId"`
}

// UserLogin 小程序登录：code 换 openid，按 openid upsert 用户后签发 token
func (s *AuthService) UserLogin(ctx context.Context, code string) (*LoginResult, error) {
	if code == "" {
		return nil, errs.BadParam("code 不能为空")
	}
	session, err := s.mp.Code2Session(ctx, code)
	if err != nil {
		return nil, errs.Wrap(errs.CodeUnauthorized, "微信登录失败", err)
	}
	var user model.User
	err = s.db.Where("openid = ?", session.OpenID).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		user = model.User{OpenID: session.OpenID, UnionID: session.UnionID, Nickname: "微信用户"}
		if err := s.db.Create(&user).Error; err != nil {
			// L7：并发登录同一新 openid 撞唯一键 → 回查复用，不再 500
			if err2 := s.db.Where("openid = ?", session.OpenID).First(&user).Error; err2 != nil {
				return nil, errs.Server(err, "创建用户失败")
			}
		}
	} else if err != nil {
		return nil, errs.Server(err, "查询用户失败")
	}
	if user.Status != 1 {
		return nil, errs.Forbidden("账号已被禁用")
	}
	token, err := s.tokens.Issue(user.ID, auth.RoleUser, "")
	if err != nil {
		return nil, errs.Server(err, "签发登录凭证失败")
	}
	var coach model.Coach
	isCoach := s.db.Where("user_id = ? AND status = 1", user.ID).First(&coach).Error == nil
	res := &LoginResult{Token: token, User: &user, IsCoach: isCoach}
	if isCoach {
		res.CoachID = coach.ID
	}
	return res, nil
}

// BindPhone 手机号快速授权绑定
func (s *AuthService) BindPhone(ctx context.Context, userID int64, phoneCode string) (string, error) {
	phone, err := s.mp.GetPhoneNumber(ctx, phoneCode)
	if err != nil {
		return "", errs.Wrap(errs.CodeBiz, "获取手机号失败", err)
	}
	if err := s.db.Model(&model.User{}).Where("id = ?", userID).Update("phone", phone).Error; err != nil {
		return "", errs.Server(err, "绑定手机号失败")
	}
	return phone, nil
}

// AdminLogin 后台账号密码登录（带失败锁定防暴力破解）
func (s *AuthService) AdminLogin(ctx context.Context, username, password, ip string) (string, *model.AdminUser, error) {
	if username == "" || password == "" {
		return "", nil, errs.BadParam("用户名或密码不能为空")
	}
	if s.lockout != nil {
		if locked, remain := s.lockout.Locked(username, ip); locked {
			return "", nil, errs.Forbidden(fmt.Sprintf("失败次数过多已锁定，请 %d 分钟后再试", int(remain.Minutes())+1))
		}
	}
	var admin model.AdminUser
	fail := func() (string, *model.AdminUser, error) {
		if s.lockout != nil {
			s.lockout.OnFail(username, ip)
		}
		return "", nil, errs.Unauthorized("用户名或密码错误")
	}
	if err := s.db.Where("username = ?", username).First(&admin).Error; err != nil {
		return fail()
	}
	if admin.Status != 1 {
		return "", nil, errs.Forbidden("账号已被禁用")
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) != nil {
		return fail()
	}
	if s.lockout != nil {
		s.lockout.OnSuccess(username, ip)
	}
	// 细分角色写进 claims（P0-1）：RequireBoss 据此放行，改角色需重登生效
	token, err := s.tokens.IssueAdmin(admin.ID, auth.RoleAdmin, admin.Username, admin.Role)
	if err != nil {
		return "", nil, errs.Server(err, "签发登录凭证失败")
	}
	return token, &admin, nil
}

// BootstrapAdmin 启动时确保存在默认老板账号（admin / 环境变量 ADMIN_INIT_PASSWORD 或 fitness123）
func (s *AuthService) BootstrapAdmin(defaultPassword string) {
	var count int64
	s.db.Model(&model.AdminUser{}).Count(&count)
	if count > 0 {
		return
	}
	if defaultPassword == "" {
		defaultPassword = "fitness123"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[bootstrap] 生成密码哈希失败: %v", err)
		return
	}
	admin := model.AdminUser{Username: "admin", PasswordHash: string(hash), Name: "老板", Role: model.AdminRoleBoss}
	if err := s.db.Create(&admin).Error; err != nil {
		log.Printf("[bootstrap] 创建默认管理员失败: %v", err)
		return
	}
	// L3：不打印明文密码；开发默认密码仅提示修改，生产密码由 ADMIN_INIT_PASSWORD 提供
	log.Printf("[bootstrap] 已创建默认管理员 admin（开发默认密码见 readme，生产请用 ADMIN_INIT_PASSWORD 并尽快修改）")
}
