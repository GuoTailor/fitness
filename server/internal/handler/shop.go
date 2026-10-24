package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fitness/internal/resp"
	"fitness/internal/service"
)

type ShopHandler struct {
	shop *service.ShopService
}

func NewShopHandler(shop *service.ShopService) *ShopHandler { return &ShopHandler{shop: shop} }

func pageOf(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	return page, size
}

func idParam(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		resp.Handle(c, service.BadParamJSON())
		return 0, false
	}
	return id, true
}

// Home GET /api/v1/home
func (h *ShopHandler) Home(c *gin.Context) {
	data, err := h.shop.Home(c.Request.Context())
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, data)
}

// ListPackages GET /api/v1/packages?category=&tag=&keyword=&page=
func (h *ShopHandler) ListPackages(c *gin.Context) {
	page, size := pageOf(c)
	items, total, err := h.shop.ListPackages(c.Request.Context(),
		c.Query("category"), c.Query("tag"), c.Query("keyword"), page, size)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"list": items, "total": total, "page": page, "pageSize": size})
}

// PackageDetail GET /api/v1/packages/:id
func (h *ShopHandler) PackageDetail(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	data, err := h.shop.PackageDetail(c.Request.Context(), id)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, data)
}

// ListCoaches GET /api/v1/coaches?specialty=&keyword=&page=
func (h *ShopHandler) ListCoaches(c *gin.Context) {
	page, size := pageOf(c)
	items, total, err := h.shop.ListCoaches(c.Request.Context(),
		c.Query("specialty"), c.Query("keyword"), page, size)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"list": items, "total": total, "page": page, "pageSize": size})
}

// CoachDetail GET /api/v1/coaches/:id
func (h *ShopHandler) CoachDetail(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	data, err := h.shop.CoachDetail(c.Request.Context(), id)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, data)
}
