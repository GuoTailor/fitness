package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"fitness/internal/errs"
	"fitness/internal/middleware"
	"fitness/internal/model"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// AdminOrderHandler 后台：订单/退款/学员
type AdminOrderHandler struct {
	db     *gorm.DB
	orders *service.OrderService
	member *service.MemberService
	audit  *service.AuditService
}

// WithAudit 装配操作审计（资金类操作留痕）
func (h *AdminOrderHandler) WithAudit(a *service.AuditService) *AdminOrderHandler {
	h.audit = a
	return h
}

func NewAdminOrderHandler(db *gorm.DB, orders *service.OrderService, member *service.MemberService) *AdminOrderHandler {
	return &AdminOrderHandler{db: db, orders: orders, member: member}
}

// ListOrders GET /admin/api/v1/orders?status=&keyword=&page=
func (h *AdminOrderHandler) ListOrders(c *gin.Context) {
	page, size := pageOf(c)
	q := h.db.Model(&model.Order{})
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	if kw := c.Query("keyword"); kw != "" {
		q = q.Where("order_no LIKE ? OR title LIKE ?", "%"+kw+"%", "%"+kw+"%")
	}
	var total int64
	q.Count(&total)
	var orders []model.Order
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&orders)
	// 补充学员昵称/手机号
	uids := make([]int64, 0, len(orders))
	for _, o := range orders {
		uids = append(uids, o.UserID)
	}
	users := map[int64]model.User{}
	if len(uids) > 0 {
		var us []model.User
		h.db.Where("id IN ?", uids).Find(&us)
		for _, u := range us {
			users[u.ID] = u
		}
	}
	type row struct {
		model.Order
		UserNickname string `json:"userNickname"`
		UserPhone    string `json:"userPhone"`
	}
	list := make([]row, 0, len(orders))
	for _, o := range orders {
		u := users[o.UserID]
		list = append(list, row{Order: o, UserNickname: u.Nickname, UserPhone: u.PlainPhone()})
	}
	resp.OK(c, gin.H{"list": list, "total": total})
}

// GetOrder GET /admin/api/v1/orders/:id（含支付流水/退款单/课时账户）
func (h *AdminOrderHandler) GetOrder(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var order model.Order
	if err := h.db.First(&order, id).Error; err != nil {
		resp.Handle(c, errs.NotFound("订单不存在"))
		return
	}
	var payments []model.Payment
	h.db.Where("order_id = ?", id).Find(&payments)
	var refunds []model.Refund
	h.db.Where("order_id = ?", id).Order("id DESC").Find(&refunds)
	var mp model.MemberPackage
	hasMP := h.db.Where("order_id = ?", id).First(&mp).Error == nil
	var user model.User
	h.db.First(&user, order.UserID)
	resp.OK(c, gin.H{
		"order": order, "payments": payments, "refunds": refunds,
		"memberPackage": mp, "hasMemberPackage": hasMP, "user": user,
	})
}

// ListRefunds GET /admin/api/v1/refunds?status=
func (h *AdminOrderHandler) ListRefunds(c *gin.Context) {
	page, size := pageOf(c)
	q := h.db.Model(&model.Refund{})
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	var total int64
	q.Count(&total)
	var refunds []model.Refund
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&refunds)
	resp.OK(c, gin.H{"list": refunds, "total": total})
}

// ApproveRefund POST /admin/api/v1/refunds/:id/approve
func (h *AdminOrderHandler) ApproveRefund(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	refund, err := h.orders.ApproveRefund(c.Request.Context(), id, middleware.AdminID(c))
	if err != nil {
		resp.Handle(c, err)
		return
	}
	h.audit.Log(c, "refund.approve", "refund", refund.ID,
		fmt.Sprintf("退款单 %s 审批通过，金额 %d 分，订单 %d", refund.RefundNo, refund.Amount, refund.OrderID))
	resp.OK(c, refund)
}

// RejectRefund POST /admin/api/v1/refunds/:id/reject {reason}
func (h *AdminOrderHandler) RejectRefund(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.orders.RejectRefund(c.Request.Context(), id, middleware.AdminID(c), req.Reason); err != nil {
		resp.Handle(c, err)
		return
	}
	h.audit.Log(c, "refund.reject", "refund", id, "拒绝退款，原因: "+req.Reason)
	resp.OK(c, gin.H{"rejected": true})
}

// ---- 学员 ----

// ListMembers GET /admin/api/v1/members?keyword=
func (h *AdminOrderHandler) ListMembers(c *gin.Context) {
	page, size := pageOf(c)
	list, total, err := h.member.ListMembersAdmin(c.Request.Context(), c.Query("keyword"), page, size)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"list": list, "total": total})
}

// MemberPackages GET /admin/api/v1/members/:id/packages
func (h *AdminOrderHandler) MemberPackages(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	list, err := h.member.MemberPackagesAdmin(c.Request.Context(), id)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, list)
}

// AdjustSessions POST /admin/api/v1/members/:id/packages/:mpId/adjust {change, remark}
func (h *AdminOrderHandler) AdjustSessions(c *gin.Context) {
	userID, ok := idParam(c, "id")
	if !ok {
		return
	}
	mpID, ok := idParam(c, "mpId")
	if !ok {
		return
	}
	var req struct {
		Change    int    `json:"change" binding:"required"`
		Remark    string `json:"remark"`
		ClientKey string `json:"clientKey"` // 客户端幂等键（弹窗打开时生成，防双击/重放重复调整）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	mp, err := h.member.AdjustSessions(c.Request.Context(), userID, mpID, req.Change, req.Remark, middleware.AdminUsername(c), req.ClientKey)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	h.audit.Log(c, "member.adjust", "member_package", mpID,
		fmt.Sprintf("手动调整 %+d 课时（学员 %d），备注: %s", req.Change, userID, req.Remark))
	resp.OK(c, mp)
}
