// Package service 业务逻辑层。
package service

import (
	"context"
	crand "crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"fitness/internal/auth"
	"fitness/internal/errs"
	"fitness/internal/middleware"
	"fitness/internal/model"
	"fitness/internal/sms"
	"fitness/internal/wechat"
)

// 短信验证码策略：5 分钟有效、60s 重发间隔、单手机单日 5 条、错 5 次作废。
// IP 层限流由路由中间件 LoginRateLimit 承担。
const (
	smsCodeTTL        = 5 * time.Minute
	smsResendInterval = 60 * time.Second
	smsDailyLimit     = 5
	smsMaxAttempts    = 5
)

var phoneRe = regexp.MustCompile(`^1\d{10}$`)

type AuthService struct {
	db          *gorm.DB
	tokens      *auth.Manager
	mp          wechat.MiniProgram
	lockout     *middleware.AdminLockout // 可为 nil（单测不启用）
	coupons     *CouponService           // 可为 nil（未接新人礼包）
	smsProvider sms.Provider             // 可为 nil（未接网页端短信登录）
	smsMock     bool
}

func NewAuthService(db *gorm.DB, tokens *auth.Manager, mp wechat.MiniProgram) *AuthService {
	return &AuthService{db: db, tokens: tokens, mp: mp}
}

// WithLockout 启用后台登录失败锁定
func (s *AuthService) WithLockout(l *middleware.AdminLockout) *AuthService {
	s.lockout = l
	return s
}

// WithCouponGifts 接入优惠券服务（新用户自动发新人礼包）
func (s *AuthService) WithCouponGifts(c *CouponService) *AuthService {
	s.coupons = c
	return s
}

// WithSms 接入短信通道（网页端手机号登录）；mock=true 时验证码由登录响应回显
func (s *AuthService) WithSms(p sms.Provider, mock bool) *AuthService {
	s.smsProvider = p
	s.smsMock = mock
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
	created := false
	err = s.db.Where("openid = ?", session.OpenID).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		user = model.User{OpenID: session.OpenID, UnionID: session.UnionID, Nickname: "微信用户"}
		if err := s.db.Create(&user).Error; err != nil {
			// L7：并发登录同一新 openid 撞唯一键 → 回查复用，不再 500
			if err2 := s.db.Where("openid = ?", session.OpenID).First(&user).Error; err2 != nil {
				return nil, errs.Server(err, "创建用户失败")
			}
		} else {
			created = true
		}
	} else if err != nil {
		return nil, errs.Server(err, "查询用户失败")
	}
	return s.issueForUser(ctx, &user, created)
}

// issueForUser 登录收尾共用逻辑：状态校验、新人礼包、签发 token、附带教练身份
func (s *AuthService) issueForUser(ctx context.Context, user *model.User, created bool) (*LoginResult, error) {
	if user.Status != 1 {
		return nil, errs.Forbidden("账号已被禁用")
	}
	if created {
		s.newcomerGifts(ctx, user.ID) // 新人礼包：失败不影响登录
	}
	token, err := s.tokens.Issue(user.ID, auth.RoleUser, "")
	if err != nil {
		return nil, errs.Server(err, "签发登录凭证失败")
	}
	var coach model.Coach
	isCoach := s.db.Where("user_id = ? AND status = 1", user.ID).First(&coach).Error == nil
	res := &LoginResult{Token: token, User: user, IsCoach: isCoach}
	if isCoach {
		res.CoachID = coach.ID
	}
	return res, nil
}

// SendLoginSms 下发网页端登录验证码。dev 返回 true 表示 mock 通道（验证码可回显给前端联调）。
func (s *AuthService) SendLoginSms(ctx context.Context, phone string) (code string, dev bool, err error) {
	if !phoneRe.MatchString(phone) {
		return "", false, errs.BadParam("手机号格式不正确")
	}
	if s.smsProvider == nil {
		return "", false, errs.Server(nil, "短信通道未启用")
	}
	// 频控 1：60s 内已发过 → 拒绝（取最新一条含已用记录判断）
	var recent model.SmsVerification
	err = s.db.WithContext(ctx).
		Where("phone = ? AND purpose = ?", phone, model.SmsPurposeLogin).
		Order("id DESC").First(&recent).Error
	if err == nil && time.Since(recent.CreatedAt) < smsResendInterval {
		return "", false, errs.Biz("发送太频繁，请稍后再试")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, errs.Server(err, "查询发送记录失败")
	}
	// 频控 2：单手机自然日上限
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var today int64
	if err := s.db.WithContext(ctx).Model(&model.SmsVerification{}).
		Where("phone = ? AND purpose = ? AND created_at >= ?", phone, model.SmsPurposeLogin, dayStart).
		Count(&today).Error; err != nil {
		return "", false, errs.Server(err, "统计发送次数失败")
	}
	if today >= smsDailyLimit {
		return "", false, errs.Biz("今日验证码发送次数已达上限")
	}
	n, err := crand.Int(crand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", false, errs.Server(err, "生成验证码失败")
	}
	code = fmt.Sprintf("%06d", n.Int64())
	v := model.SmsVerification{
		Phone: phone, Code: code, Purpose: model.SmsPurposeLogin,
		ExpiresAt: now.Add(smsCodeTTL),
	}
	if err := s.db.WithContext(ctx).Create(&v).Error; err != nil {
		return "", false, errs.Server(err, "保存验证码失败")
	}
	if err := s.smsProvider.Send(ctx, phone, code); err != nil {
		return "", false, errs.Wrap(errs.CodeBiz, "短信发送失败", err)
	}
	return code, s.smsMock, nil
}

// LoginByPhone 网页端手机号+验证码登录：验证通过后按手机号 upsert 用户（已绑定该
// 手机的微信用户登录即同一账号；全新手机号建 web 用户，openid 占位 web-{phone}）。
func (s *AuthService) LoginByPhone(ctx context.Context, phone, smsCode string) (*LoginResult, error) {
	if !phoneRe.MatchString(phone) {
		return nil, errs.BadParam("手机号格式不正确")
	}
	if strings.TrimSpace(smsCode) == "" {
		return nil, errs.BadParam("验证码不能为空")
	}
	var v model.SmsVerification
	err := s.db.WithContext(ctx).
		Where("phone = ? AND purpose = ? AND used_at IS NULL", phone, model.SmsPurposeLogin).
		Order("id DESC").First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.Biz("验证码无效，请重新获取")
	}
	if err != nil {
		return nil, errs.Server(err, "查询验证码失败")
	}
	if time.Now().After(v.ExpiresAt) {
		return nil, errs.Biz("验证码已过期，请重新获取")
	}
	if v.Attempts >= smsMaxAttempts {
		return nil, errs.Biz("尝试次数过多，请重新获取验证码")
	}
	// 常量时间比较防时序侧信道；失配计一次尝试（防暴力猜码）
	if subtle.ConstantTimeCompare([]byte(smsCode), []byte(v.Code)) != 1 {
		if err := s.db.WithContext(ctx).Model(&model.SmsVerification{}).
			Where("id = ?", v.ID).
			UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error; err != nil {
			log.Printf("[sms] 记录验证码尝试次数失败: %v", err)
		}
		return nil, errs.Biz("验证码错误")
	}
	// 用后作废：条件更新防并发重放同一验证码
	res := s.db.WithContext(ctx).Model(&model.SmsVerification{}).
		Where("id = ? AND used_at IS NULL", v.ID).Update("used_at", time.Now())
	if res.Error != nil {
		return nil, errs.Server(res.Error, "核销验证码失败")
	}
	if res.RowsAffected == 0 {
		return nil, errs.Biz("验证码已被使用")
	}

	var user model.User
	created := false
	err = s.db.Where("phone = ?", phone).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		openid := "web-" + phone // 占位 openid：与微信 openid 空间隔离，保证唯一索引可用
		nick := "用户" + phone[len(phone)-4:]
		user = model.User{OpenID: openid, Phone: &phone, Nickname: nick}
		if err := s.db.Create(&user).Error; err != nil {
			// 并发登录同一新手机号撞唯一键 → 回查复用
			if err2 := s.db.Where("phone = ?", phone).First(&user).Error; err2 != nil {
				return nil, errs.Server(err, "创建用户失败")
			}
		} else {
			created = true
		}
	} else if err != nil {
		return nil, errs.Server(err, "查询用户失败")
	}
	return s.issueForUser(ctx, &user, created)
}

// newcomerGifts 新用户自动发放新人礼包（marketing.newcomer_gift_templates，
// CSV 券模板 ID；空配置不发放；单券失败只记日志）
func (s *AuthService) newcomerGifts(ctx context.Context, userID int64) {
	if s.coupons == nil {
		return
	}
	raw := NewConfigService(s.db).Get(ctx, model.CfgNewcomerGiftTemplates, "")
	if strings.TrimSpace(raw) == "" {
		return
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		if _, err := s.coupons.Grant(ctx, userID, id); err != nil {
			log.Printf("[newcomer-gift] 用户 %d 发放券模板 %d 失败: %v", userID, id, err)
		}
	}
}

// BindPhone 手机号快速授权绑定（phone 全局唯一：一人一号，新人价判重的数据前提）
func (s *AuthService) BindPhone(ctx context.Context, userID int64, phoneCode string) (string, error) {
	phone, err := s.mp.GetPhoneNumber(ctx, phoneCode)
	if err != nil {
		return "", errs.Wrap(errs.CodeBiz, "获取手机号失败", err)
	}
	if phone == "" {
		return "", errs.BadParam("手机号为空")
	}
	var dup int64
	if err := s.db.WithContext(ctx).Model(&model.User{}).
		Where("phone = ? AND id <> ?", phone, userID).Count(&dup).Error; err != nil {
		return "", errs.Server(err, "校验手机号失败")
	}
	if dup > 0 {
		return "", errs.Biz("该手机号已绑定其他账号")
	}
	err = s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("phone", phone).Error
	if isDuplicateKey(err) { // 唯一索引兜底（并发绑定同一号）
		return "", errs.Biz("该手机号已绑定其他账号")
	}
	if err != nil {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fail()
		}
		// 数据库异常不当作密码错误（否则连接抖动会污染失败锁定计数）
		return "", nil, errs.Server(err, "登录服务暂不可用，请稍后再试")
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
