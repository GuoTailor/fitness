package handler

import (
	"github.com/gin-gonic/gin"

	"fitness/internal/resp"
	"fitness/internal/service"
)

// StatsHandler 管理后台数据看板
type StatsHandler struct{ stats *service.StatsService }

func NewStatsHandler(s *service.StatsService) *StatsHandler { return &StatsHandler{stats: s} }

// Dashboard GET /admin/api/v1/dashboard
func (h *StatsHandler) Dashboard(c *gin.Context) {
	data, err := h.stats.Dashboard(c.Request.Context())
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, data)
}
