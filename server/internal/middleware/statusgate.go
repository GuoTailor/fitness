package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/model"
	"fitness/internal/resp"
)

// 账号禁用即时生效：无状态 JWT 本身无法吊销，这里在验签通过后校验账号状态
// （user/admin_user.status），配合 60s 内存缓存把每账号 DB 查询压到每分钟一次。
// 禁用账号的存量 token 最迟 60s 内全端失效（传播窗口记录于进度记录）。
// DB 查询失败按放行处理（fail-open，不缓存）——避免闪断把全体用户锁在门外。

const statusCacheTTL = 60 * time.Second

type statusEntry struct {
	ok bool
	at time.Time
}

var tokenStatus = struct {
	mu      sync.Mutex
	entries map[string]statusEntry
}{entries: map[string]statusEntry{}}

// StatusGate 挂在 JWT 之后：校验已认证 token 对应账号是否仍启用
func StatusGate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := claimsOf(c)
		if claims == nil || claims.UserID <= 0 {
			c.Next()
			return
		}
		if !accountActive(db, claims.Role, claims.UserID) {
			resp.Handle(c, errs.Unauthorized("账号已被禁用"))
			c.Abort()
			return
		}
		c.Next()
	}
}

func accountActive(db *gorm.DB, role string, uid int64) bool {
	key := fmt.Sprintf("%s:%d", role, uid)
	now := time.Now()
	tokenStatus.mu.Lock()
	if e, ok := tokenStatus.entries[key]; ok && now.Sub(e.at) < statusCacheTTL {
		tokenStatus.mu.Unlock()
		return e.ok
	}
	tokenStatus.mu.Unlock()

	ok, known := false, false
	switch role {
	case "user":
		var n int64
		if err := db.Model(&model.User{}).Where("id = ? AND status = 1", uid).Count(&n).Error; err == nil {
			ok, known = n > 0, true
		}
	case "admin":
		var n int64
		if err := db.Model(&model.AdminUser{}).Where("id = ? AND status = 1", uid).Count(&n).Error; err == nil {
			ok, known = n > 0, true
		}
	default:
		ok, known = true, true // 未知角色不拦截
	}
	if !known {
		return true // DB 异常 fail-open：本次放行，下次请求重试
	}
	tokenStatus.mu.Lock()
	tokenStatus.entries[key] = statusEntry{ok: ok, at: now}
	// 惰性清理：防止 key 无限增长
	if len(tokenStatus.entries) > 4096 {
		for k, e := range tokenStatus.entries {
			if now.Sub(e.at) > statusCacheTTL {
				delete(tokenStatus.entries, k)
			}
		}
	}
	tokenStatus.mu.Unlock()
	return ok
}
