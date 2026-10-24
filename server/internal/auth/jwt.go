// Package auth JWT 签发与校验。角色：user（学员，含双角色教练）、admin。
// 教练身份通过 coach.user_id 绑定关系即时判定，不进 token，避免解绑后旧 token 仍带权限。
package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"fitness/internal/errs"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

type Claims struct {
	UserID   int64  `json:"uid"`
	Role     string `json:"role"`
	Username string `json:"username,omitempty"` // admin 用
	jwt.RegisteredClaims
}

type Manager struct {
	secret      []byte
	expireHours int
}

func NewManager(secret string, expireHours int) *Manager {
	return &Manager{secret: []byte(secret), expireHours: expireHours}
}

func (m *Manager) Issue(userID int64, role, username string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Role:     role,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(m.expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *Manager) Parse(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errs.Unauthorized("非法 token")
		}
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, errs.Unauthorized("登录已失效，请重新登录")
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errs.Unauthorized("非法 token")
	}
	return claims, nil
}
