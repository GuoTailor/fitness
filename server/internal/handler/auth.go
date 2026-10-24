// Package handler HTTP 处理层（api=学员端+教练端，admin=管理后台）。
package handler

import (
	"github.com/gin-gonic/gin"

	"fitness/internal/middleware"
	"fitness/internal/resp"
	"fitness/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// Login POST /api/v1/auth/login {code}
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	res, err := h.auth.UserLogin(c.Request.Context(), req.Code)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, res)
}

// BindPhone POST /api/v1/auth/phone {code}（手机号快速验证组件 phone code）
func (h *AuthHandler) BindPhone(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	phone, err := h.auth.BindPhone(c.Request.Context(), middleware.UserID(c), req.Code)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"phone": phone})
}

// AdminLogin POST /admin/api/v1/auth/login {username,password}
func (h *AuthHandler) AdminLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	token, admin, err := h.auth.AdminLogin(c.Request.Context(), req.Username, req.Password, c.ClientIP())
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"token": token, "admin": admin})
}
