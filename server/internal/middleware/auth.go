// Package middleware 认证/角色守卫/CORS。
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"fitness/internal/auth"
	"fitness/internal/errs"
	"fitness/internal/model"
	"fitness/internal/resp"
)

const CtxClaims = "claims"

// JWT 解析 Bearer token 并挂载 claims（公开接口也走这里，未带 token 不拦截）
func JWT(m *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if h == "" {
			c.Next()
			return
		}
		token := strings.TrimPrefix(h, "Bearer ")
		if token == h { // 没有 Bearer 前缀时直接按裸 token 处理
			token = h
		}
		claims, err := m.Parse(token)
		if err != nil {
			resp.Handle(c, err)
			c.Abort()
			return
		}
		c.Set(CtxClaims, claims)
		c.Next()
	}
}

func claimsOf(c *gin.Context) *auth.Claims {
	v, _ := c.Get(CtxClaims)
	claims, _ := v.(*auth.Claims)
	return claims
}

// UserID 取当前登录学员 ID（未登录 0）
func UserID(c *gin.Context) int64 {
	if claims := claimsOf(c); claims != nil && claims.Role == auth.RoleUser {
		return claims.UserID
	}
	return 0
}

// AdminID 取当前后台账号 ID（未登录 0）
func AdminID(c *gin.Context) int64 {
	if claims := claimsOf(c); claims != nil && claims.Role == auth.RoleAdmin {
		return claims.UserID
	}
	return 0
}

// AdminUsername 取当前后台账号用户名
func AdminUsername(c *gin.Context) string {
	if claims := claimsOf(c); claims != nil && claims.Role == auth.RoleAdmin {
		return claims.Username
	}
	return "admin"
}

// RequireUser 学员端登录（教练双角色复用学员身份）
func RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := claimsOf(c)
		if claims == nil || claims.Role != auth.RoleUser {
			resp.Handle(c, errs.Unauthorized("请先登录"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireCoach 教练工作台：已登录学员且绑定了在职教练身份。
// 教练身份不入 token，按 coach.user_id 即时判定，解绑立即失效。
func RequireCoach(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := claimsOf(c)
		if claims == nil || claims.Role != auth.RoleUser {
			resp.Handle(c, errs.Unauthorized("请先登录"))
			c.Abort()
			return
		}
		var coach model.Coach
		err := db.Where("user_id = ? AND status = 1", claims.UserID).First(&coach).Error
		if err != nil {
			resp.Handle(c, errs.Forbidden("未开通教练工作台"))
			c.Abort()
			return
		}
		c.Set("coach_id", coach.ID)
		c.Next()
	}
}

// CoachID 取当前教练 ID（非教练上下文 0）
func CoachID(c *gin.Context) int64 {
	v, _ := c.Get("coach_id")
	id, _ := v.(int64)
	return id
}

// RequireAdmin 后台登录（boss/receptionist 均可，细粒度用 AdminRole 判断）
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := claimsOf(c)
		if claims == nil || claims.Role != auth.RoleAdmin {
			resp.Handle(c, errs.Unauthorized("请先登录后台"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireBoss 仅老板（退款审批/课时调整/运营配置等资金与规则类操作）
func RequireBoss() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := claimsOf(c)
		if claims == nil || claims.Role != auth.RoleAdmin || claims.AdminRole != model.AdminRoleBoss {
			resp.Handle(c, errs.Forbidden("需要老板权限"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// CORS 白名单放行（L5：不再反射任意 Origin；不在名单内的一律不加跨域头）。
// 小程序请求无跨域问题；admin-web 开发走 vite 代理也可不用 CORS。
func CORS(allowed []string) gin.HandlerFunc {
	allowSet := map[string]bool{}
	for _, o := range allowed {
		allowSet[o] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowSet[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
