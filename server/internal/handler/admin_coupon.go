package handler

import (
	"github.com/gin-gonic/gin"

	"fitness/internal/errs"
	"fitness/internal/model"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// AdminCouponHandler 后台优惠券：列表（含领取/使用对账）、新建、停用/编辑、定向发放
type AdminCouponHandler struct {
	coupons *service.CouponService
}

func NewAdminCouponHandler(c *service.CouponService) *AdminCouponHandler {
	return &AdminCouponHandler{coupons: c}
}

// List GET /admin/api/v1/coupons
func (h *AdminCouponHandler) List(c *gin.Context) {
	var tpls []model.CouponTemplate
	if err := h.coupons.DB().Order("id DESC").Limit(200).Find(&tpls).Error; err != nil {
		resp.Handle(c, err)
		return
	}
	type stat struct {
		TemplateID int64
		Held       int64
		Used       int64
	}
	var stats []stat
	h.coupons.DB().Model(&model.UserCoupon{}).
		Select("template_id, COUNT(*) AS held, SUM(CASE WHEN status = 'used' THEN 1 ELSE 0 END) AS used").
		Group("template_id").Scan(&stats)
	held := map[int64]int64{}
	used := map[int64]int64{}
	for _, s := range stats {
		held[s.TemplateID] = s.Held
		used[s.TemplateID] = s.Used
	}
	list := make([]map[string]interface{}, 0, len(tpls))
	for i := range tpls {
		t := &tpls[i]
		list = append(list, map[string]interface{}{
			"id": t.ID, "name": t.Name, "type": t.Type, "label": service.TemplateLabel(t),
			"threshold": t.Threshold, "amount": t.Amount, "packageId": t.PackageID,
			"totalCount": t.TotalCount, "issuedCount": t.IssuedCount,
			"perUserLimit": t.PerUserLimit, "validDays": t.ValidDays,
			"status": t.Status, "remark": t.Remark, "createdAt": t.CreatedAt,
			"heldCount": held[t.ID], "usedCount": used[t.ID],
		})
	}
	resp.OK(c, gin.H{"list": list})
}

type couponPayload struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Threshold    int64  `json:"threshold"`
	Amount       int64  `json:"amount"`
	PackageID    int64  `json:"packageId"`
	TotalCount   int    `json:"totalCount"`
	PerUserLimit int    `json:"perUserLimit"`
	ValidDays    int    `json:"validDays"`
	Status       int8   `json:"status"`
	Remark       string `json:"remark"`
}

// Save POST /admin/api/v1/coupons（新建）/ :id（编辑）。
// 已有发放量的券不允许改优惠力度（amount/type），防止已领券权益被事后篡改。
func (h *AdminCouponHandler) Save(c *gin.Context) {
	var req couponPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	db := h.coupons.DB()
	tpl := model.CouponTemplate{
		Name: req.Name, Type: req.Type, Threshold: req.Threshold, Amount: req.Amount,
		PackageID: req.PackageID, TotalCount: req.TotalCount, PerUserLimit: req.PerUserLimit,
		ValidDays: req.ValidDays, Status: req.Status, Remark: req.Remark,
	}
	// :id 为可选（新建无）——idParam 缺参时会写响应，这里手动判空
	isCreate := true
	if idStr := c.Param("id"); idStr != "" {
		id, ok := idParam(c, "id")
		if !ok {
			return
		}
		var exist model.CouponTemplate
		if err := db.First(&exist, id).Error; err != nil {
			resp.Handle(c, errs.NotFound("券不存在"))
			return
		}
		if exist.IssuedCount > 0 && (exist.Amount != tpl.Amount || exist.Type != tpl.Type) {
			resp.Handle(c, errs.BadParam("已发放的券不允许修改优惠类型或力度，请新建一张"))
			return
		}
		tpl.ID = exist.ID
		tpl.OwnerID = exist.OwnerID
		tpl.IssuedCount = exist.IssuedCount // 发放量只增不减
		tpl.Status = exist.Status           // 启停走 Toggle 单独管理
		isCreate = false
	}
	if isCreate {
		tpl.Status = 1 // 新建即启用
	}
	if tpl.PerUserLimit == 0 {
		tpl.PerUserLimit = 1
	}
	if tpl.ValidDays == 0 {
		tpl.ValidDays = 30
	}
	if err := service.ValidateTemplate(&tpl); err != nil {
		resp.Handle(c, err)
		return
	}
	if err := db.Save(&tpl).Error; err != nil {
		resp.Handle(c, errs.Server(err, "保存券失败"))
		return
	}
	resp.OK(c, tpl)
}

// Toggle POST /admin/api/v1/coupons/:id/status {status:0|1}
func (h *AdminCouponHandler) Toggle(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Status int8 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Status != 0 && req.Status != 1) {
		resp.Handle(c, errs.BadParam("status 仅支持 0/1"))
		return
	}
	res := h.coupons.DB().Model(&model.CouponTemplate{}).
		Where("id = ?", id).Update("status", req.Status)
	if res.Error != nil || res.RowsAffected == 0 {
		resp.Handle(c, errs.NotFound("券不存在"))
		return
	}
	resp.OK(c, gin.H{"updated": true})
}

// Grant POST /admin/api/v1/coupons/:id/grant {userId} 定向发放（绕过每人限领）
func (h *AdminCouponHandler) Grant(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		UserID int64 `json:"userId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	uc, err := h.coupons.Grant(c.Request.Context(), req.UserID, id)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, uc)
}
