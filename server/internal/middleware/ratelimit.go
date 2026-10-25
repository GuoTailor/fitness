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
	calls   uint64 // 定期触发全量清扫（E12：IP 众多时懒删不足以回收 map 空间）
}

func newSlidingWindow(limit int, window time.Duration) *slidingWindow {
	return &slidingWindow{events: map[string][]time.Time{}, limit: limit, window: window}
}

// sweep 清除所有键中窗口外的记录，删除空键（内存回收）
func (w *slidingWindow) sweep() {
	cutoff := time.Now().Add(-w.window)
	for k, list := range w.events {
		kept := list[:0]
		for _, t := range list {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(w.events, k)
		} else {
			w.events[k] = kept
		}
	}
}

func (w *slidingWindow) allow(key string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-w.window)
	list := w.events[key]
	// 懒删除窗口外记录
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
	// 每 1024 次调用做一次全量清扫，防止大量一次性 IP 让 map 无界增长
	w.calls++
	if w.calls%1024 == 0 {
		w.sweep()
	}
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

// Locked 该账号+IP 是否处于锁定期（返回剩余时间）；顺带回收过期锁定条目（E12）
func (l *AdminLockout) Locked(username, ip string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	k := l.key(username, ip)
	last, ok := l.lastFail[k]
	if ok && l.fails[k] >= l.threshold {
		if elapsed := time.Since(last); elapsed < l.lockTime {
			return true, l.lockTime - elapsed
		}
		delete(l.fails, k)
		delete(l.lastFail, k)
	}
	// 周期回收：超过锁定时长的旧条目不再占内存（上限清扫 256 条/次，避免持锁过久）
	if len(l.fails) > 64 {
		swept := 0
		for fk, ft := range l.lastFail {
			if time.Since(ft) > l.lockTime {
				delete(l.fails, fk)
				delete(l.lastFail, fk)
				swept++
				if swept >= 256 {
					break
				}
			}
		}
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
