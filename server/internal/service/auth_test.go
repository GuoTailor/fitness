package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"fitness/internal/auth"
	"fitness/internal/db"
	"fitness/internal/model"
	"fitness/internal/wechat"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	d, err := db.OpenSQLite()
	require.NoError(t, err)
	require.NoError(t, d.AutoMigrate(
		&model.User{}, &model.Coach{}, &model.AdminUser{},
	))
	return d
}

func newAuthSvc(t *testing.T) (*AuthService, *gorm.DB) {
	t.Helper()
	d := setupTestDB(t)
	tokens := auth.NewManager("test-secret", 1)
	return NewAuthService(d, tokens, wechat.NewMock()), d
}

func TestUserLoginMock(t *testing.T) {
	svc, _ := newAuthSvc(t)
	ctx := context.Background()

	// 首次登录自动建用户
	res, err := svc.UserLogin(ctx, "mock-user-1")
	require.NoError(t, err)
	assert.NotEmpty(t, res.Token)
	assert.Equal(t, "mock-openid-mock-user-1", res.User.OpenID)
	assert.False(t, res.IsCoach, "普通学员不应有教练身份")

	// 同 code 再登录复用同一账号
	res2, err := svc.UserLogin(ctx, "mock-user-1")
	require.NoError(t, err)
	assert.Equal(t, res.User.ID, res2.User.ID, "同一 openid 应复用用户")

	// token 可解析且角色正确
	claims, err := auth.NewManager("test-secret", 1).Parse(res.Token)
	require.NoError(t, err)
	assert.Equal(t, res.User.ID, claims.UserID)
	assert.Equal(t, auth.RoleUser, claims.Role)
}

func TestUserLoginCoachRole(t *testing.T) {
	svc, d := newAuthSvc(t)
	ctx := context.Background()

	res, err := svc.UserLogin(ctx, "mock-coach-1")
	require.NoError(t, err)

	// 后台把该学员绑定为教练后，登录应返回 isCoach
	coach := model.Coach{UserID: res.User.ID, Name: "教练A", Status: 1}
	require.NoError(t, d.Create(&coach).Error)
	res2, err := svc.UserLogin(ctx, "mock-coach-1")
	require.NoError(t, err)
	assert.True(t, res2.IsCoach)
	assert.Equal(t, coach.ID, res2.CoachID)

	// 停用后即时失活
	require.NoError(t, d.Model(&coach).Update("status", 0).Error)
	res3, err := svc.UserLogin(ctx, "mock-coach-1")
	require.NoError(t, err)
	assert.False(t, res3.IsCoach, "教练停用后登录不应再带教练身份")
}

func TestBindPhone(t *testing.T) {
	svc, _ := newAuthSvc(t)
	ctx := context.Background()

	res, _ := svc.UserLogin(ctx, "mock-user-2")
	phone, err := svc.BindPhone(ctx, res.User.ID, "13800138000")
	require.NoError(t, err)
	assert.Equal(t, "13800138000", phone)
}

func TestAdminLogin(t *testing.T) {
	svc, d := newAuthSvc(t)
	ctx := context.Background()

	hash, err := bcrypt.GenerateFromPassword([]byte("right-pass"), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, d.Create(&model.AdminUser{
		Username: "boss", PasswordHash: string(hash), Name: "老板", Role: model.AdminRoleBoss,
	}).Error)

	// 正确密码
	token, admin, err := svc.AdminLogin(ctx, "boss", "right-pass", "127.0.0.1")
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, model.AdminRoleBoss, admin.Role)

	// 错误密码
	_, _, err = svc.AdminLogin(ctx, "boss", "wrong-pass", "127.0.0.1")
	assert.Error(t, err)

	// 不存在用户
	_, _, err = svc.AdminLogin(ctx, "ghost", "x", "127.0.0.1")
	assert.Error(t, err)
}

func TestBootstrapAdminIdempotent(t *testing.T) {
	svc, d := newAuthSvc(t)
	svc.BootstrapAdmin("init-pass")
	var count int64
	d.Model(&model.AdminUser{}).Count(&count)
	require.Equal(t, int64(1), count)

	svc.BootstrapAdmin("init-pass")
	d.Model(&model.AdminUser{}).Count(&count)
	assert.Equal(t, int64(1), count, "重复 bootstrap 不得重复建号")

	// 默认账号可登录
	_, _, err := svc.AdminLogin(context.Background(), "admin", "init-pass", "127.0.0.1")
	assert.NoError(t, err)
}
