// 登录与安全：IP 滑窗限流 + 管理后台登录失败锁定（防暴力破解，内存实现，单实例够用）。
package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"fitness/internal/errs"
	"fitness/internal/resp"
)

type slidingWindow struct {
	mu      sync.Mutex
	events  map[string][]time.Time
	limit   int
	window  time.Duration
}

func newSlidingWindow(limit int, window time.Duration) *slidingWindow {
	return &slidingWindow{events: map[string][]time.Time{}, limit: limit, window: window}
}

func (w *slidingWindow) allow(key string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-w.window)
	list := w.events[key]
	// 淘汰窗口外记录
	kept := list[:0]
	for _, t := range list {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= w.limit {
		w.events[key] = kept
		return false
	}
	w.events[key] = append(kept, now)
	return true
}

// clientIP 取真实来源（P0 未上反代，直取 RemoteAddr；nginx 部署后切 X-Forwarded-For）
func clientIP(c *gin.Context) string {
	if ip := c.ClientIP(); ip != "" {
		return ip
	}
	return "unknown"
}

// LoginRateLimit 登录类接口 IP 限流：默认 10 次/分钟
func LoginRateLimit(limitPerMin int) gin.HandlerFunc {
	w := newSlidingWindow(limitPerMin, time.Minute)
	return func(c *gin.Context) {
		if !w.allow("ip:" + clientIP(c)) {
			resp.Handle(c, errs.New(errs.CodeForbidden, "操作过于频繁，请稍后再试"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// AdminLockout 后台登录失败锁定：同一 用户名+IP 连续失败 N 次锁 M 分钟
type AdminLockout struct {
	mu       sync.Mutex
	fails    map[string]int
	lastFail map[string]time.Time
	threshold int
	lockTime  time.Duration
}

func NewAdminLockout(threshold int, lockMinutes int) *AdminLockout {
	return &AdminLockout{
		fails: map[string]int{}, lastFail: map[string]time.Time{},
		threshold: threshold, lockTime: time.Duration(lockMinutes) * time.Minute,
	}
}

func (l *AdminLockout) key(username, ip string) string { return username + "@" + ip }

// Locked 该账号+IP 是否处于锁定期（返回剩余时间）
func (l *AdminLockout) Locked(username, ip string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	last, ok := l.lastFail[l.key(username, ip)]
	if ok && l.fails[l.key(username, ip)] >= l.threshold {
		if elapsed := time.Since(last); elapsed < l.lockTime {
			return true, l.lockTime - elapsed
		}
		delete(l.fails, l.key(username, ip))
		delete(l.lastFail, l.key(username, ip))
	}
	return false, 0
}

func (l *AdminLockout) OnFail(username, ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	k := l.key(username, ip)
	l.fails[k]++
	l.lastFail[k] = time.Now()
}

func (l *AdminLockout) OnSuccess(username, ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	k := l.key(username, ip)
	delete(l.fails, k)
	delete(l.lastFail, k)
}
