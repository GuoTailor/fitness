package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"fitness/internal/middleware"
	"fitness/internal/model"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// MeHandler GET /api/v1/me 当前用户信息（含教练身份）
type MeHandler struct {
	db *gorm.DB
}

func NewMeHandler(db *gorm.DB) *MeHandler { return &MeHandler{db: db} }

func (h *MeHandler) Me(c *gin.Context) {
	uid := middleware.UserID(c)
	var user model.User
	if err := h.db.First(&user, uid).Error; err != nil {
		resp.Handle(c, service.NotFoundJSON())
		return
	}
	var coach model.Coach
	isCoach := h.db.Where("user_id = ? AND status = 1", uid).First(&coach).Error == nil
	resp.OK(c, gin.H{
		"user":    user,
		"isCoach": isCoach,
		"coachId": coach.ID,
	})
}

// Ping GET /healthz 无需登录，用于探活
func Ping(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0, "msg": "pong", "data": gin.H{"service": "fitness-server"}})
}
