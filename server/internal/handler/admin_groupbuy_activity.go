package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"fitness/internal/errs"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// AdminGroupBuyActivityHandler 后台拼团限时活动：列表、新建/编辑、删除。
// 变更类操作与优惠券同款门控（路由层 RequireBoss）+ 审计。
type AdminGroupBuyActivityHandler struct {
	svc   *service.GroupBuyService
	audit *service.AuditService
}

func NewAdminGroupBuyActivityHandler(svc *service.GroupBuyService) *AdminGroupBuyActivityHandler {
	return &AdminGroupBuyActivityHandler{svc: svc}
}

func (h *AdminGroupBuyActivityHandler) WithAudit(a *service.AuditService) *AdminGroupBuyActivityHandler {
	h.audit = a
	return h
}

// List GET /admin/api/v1/group-buy-activities
func (h *AdminGroupBuyActivityHandler) List(c *gin.Context) {
	list, err := h.svc.ActivityList(c.Request.Context())
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"list": list})
}

type groupBuyActivityPayload struct {
	ID            int64  `json:"id"`
	Title         string `json:"title" binding:"required"`
	SkuID         int64  `json:"skuId" binding:"required"`
	GroupPrice    int64  `json:"groupPrice"`
	GroupCapacity int    `json:"groupCapacity"`
	StartAt       string `json:"startAt"`
	EndAt         string `json:"endAt"`
	MaxGroups     int    `json:"maxGroups"`
	ExpireHours   int    `json:"expireHours"`
	Status        int8   `json:"status"`
}

// Save POST /admin/api/v1/group-buy-activities（新建）/ :id（编辑）
func (h *AdminGroupBuyActivityHandler) Save(c *gin.Context) {
	var req groupBuyActivityPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	in := service.ActivityInput{
		ID: req.ID, Title: req.Title, SkuID: req.SkuID,
		GroupPrice: req.GroupPrice, GroupCapacity: req.GroupCapacity,
		MaxGroups: req.MaxGroups, ExpireHours: req.ExpireHours, Status: req.Status,
	}
	if in.Status != 0 && in.Status != 1 {
		resp.Handle(c, errs.BadParam("status 仅支持 0/1"))
		return
	}
	var err error
	var startAt, endAt *time.Time
	if startAt, err = parsePromoTime(req.StartAt); err != nil {
		resp.Handle(c, errs.BadParam("开始时间格式不正确"))
		return
	}
	if endAt, err = parsePromoTime(req.EndAt); err != nil {
		resp.Handle(c, errs.BadParam("结束时间格式不正确"))
		return
	}
	if startAt != nil {
		in.StartAt = *startAt
	}
	if endAt != nil {
		in.EndAt = *endAt
	}
	act, err := h.svc.ActivitySave(c.Request.Context(), &in)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	h.audit.Log(c, "groupbuy_activity.save", "group_buy_activity", act.ID,
		fmt.Sprintf("保存拼团活动「%s」sku=%d 价=%d %d人团", act.Title, act.SkuID, act.GroupPrice, act.GroupCapacity))
	resp.OK(c, act)
}

// Delete DELETE /admin/api/v1/group-buy-activities/:id（仅未开团的允许删）
func (h *AdminGroupBuyActivityHandler) Delete(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.ActivityDelete(c.Request.Context(), id); err != nil {
		resp.Handle(c, err)
		return
	}
	h.audit.Log(c, "groupbuy_activity.delete", "group_buy_activity", id,
		fmt.Sprintf("删除拼团活动 %d", id))
	resp.OK(c, gin.H{"deleted": true})
}
