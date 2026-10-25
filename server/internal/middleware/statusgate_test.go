package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fitness/internal/db"
	"fitness/internal/model"
)

// 账号禁用即时生效：启用账号放行（可缓存），禁用账号 60s 内失效；DB 失败 fail-open 不锁人
func TestAccountActive(t *testing.T) {
	d, err := db.OpenSQLite()
	require.NoError(t, err)
	require.NoError(t, d.AutoMigrate(&model.User{}, &model.AdminUser{}))
	require.NoError(t, d.Create(&model.User{ID: 1, OpenID: "ou-sg", Status: 1}).Error)
	require.NoError(t, d.Create(&model.User{ID: 2, OpenID: "ou-sg2"}).Error)
	// Status 带 default:1，零值建行会被 GORM 忽略为默认 1，禁用态需显式更新
	require.NoError(t, d.Model(&model.User{}).Where("id = ?", 2).Update("status", 0).Error)
	require.NoError(t, d.Create(&model.AdminUser{ID: 5, Username: "a"}).Error)

	// 清空缓存避免用例间串扰
	tokenStatus.entries = map[string]statusEntry{}

	assert.True(t, accountActive(d, "user", 1), "正常用户放行")
	assert.False(t, accountActive(d, "user", 2), "禁用用户拦截")
	assert.False(t, accountActive(d, "user", 999), "不存在用户拦截")
	assert.True(t, accountActive(d, "admin", 5), "正常管理员放行")

	// 缓存生效：禁用后再查（未过期）仍读缓存值
	require.NoError(t, d.Model(&model.User{}).Where("id = ?", 1).Update("status", 0).Error)
	assert.True(t, accountActive(d, "user", 1), "TTL 内读缓存（60s 传播窗口）")

	// 过期后重新查询
	tokenStatus.entries["user:1"] = statusEntry{ok: true, at: time.Now().Add(-statusCacheTTL - time.Second)}
	assert.False(t, accountActive(d, "user", 1), "缓存过期后读到禁用状态")
}

// 未知角色 fail-open
func TestAccountActiveUnknownRole(t *testing.T) {
	d, err := db.OpenSQLite()
	require.NoError(t, err)
	assert.True(t, accountActive(d, "unknown-role", 12345))
}
