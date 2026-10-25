package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"fitness/internal/config"
	"fitness/internal/errs"
	"fitness/internal/model"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// AdminShopHandler 后台：课包/SKU/教练/Banner/配置
type AdminShopHandler struct {
	db    *gorm.DB
	cfg   *service.ConfigService
	owner int64
}

func NewAdminShopHandler(db *gorm.DB, cfg *service.ConfigService, appCfg *config.Config) *AdminShopHandler {
	return &AdminShopHandler{db: db, cfg: cfg, owner: appCfg.OwnerID}
}

// ---- 课包 ----

type skuPayload struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Sessions      int    `json:"sessions"`
	ValidDays     int    `json:"validDays"`
	Price         int64  `json:"price"`
	OriginalPrice int64  `json:"originalPrice"`
	NewcomerPrice int64  `json:"newcomerPrice"` // 新人价（分），0=不启用
	Status        int8   `json:"status"`
	Sort          int    `json:"sort"`
}

type packagePayload struct {
	Title         string       `json:"title" binding:"required"`
	CoverURL      string       `json:"coverUrl"`
	IntroVideoURL string       `json:"introVideoUrl"`
	Category      string       `json:"category" binding:"required"`
	Tags          string       `json:"tags"`
	Difficulty    int8         `json:"difficulty"`
	Outline       string       `json:"outline"`
	Suitable      string       `json:"suitable"`
	Status        int8         `json:"status"`
	Sort          int          `json:"sort"`
	Skus          []skuPayload `json:"skus"`
	CoachIDs      []int64      `json:"coachIds"`
}

// ListPackages GET /admin/api/v1/packages?status=&keyword=
func (h *AdminShopHandler) ListPackages(c *gin.Context) {
	page, size := pageOf(c)
	q := h.db.Model(&model.CoursePackage{}).Where("owner_id = ?", h.owner)
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	if kw := c.Query("keyword"); kw != "" {
		q = q.Where("title LIKE ?", "%"+kw+"%")
	}
	var total int64
	q.Count(&total)
	var pkgs []model.CoursePackage
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&pkgs)
	// 附带 skus 与教练ID，前端一次拉全
	ids := make([]int64, 0, len(pkgs))
	for _, p := range pkgs {
		ids = append(ids, p.ID)
	}
	skuMap := map[int64][]model.PackageSKU{}
	coachMap := map[int64][]int64{}
	if len(ids) > 0 {
		var skus []model.PackageSKU
		h.db.Where("package_id IN ?", ids).Order("sort, id").Find(&skus)
		for _, s := range skus {
			skuMap[s.PackageID] = append(skuMap[s.PackageID], s)
		}
		var cps []model.CoachPackage
		h.db.Where("package_id IN ?", ids).Find(&cps)
		for _, cp := range cps {
			coachMap[cp.PackageID] = append(coachMap[cp.PackageID], cp.CoachID)
		}
	}
	type row struct {
		model.CoursePackage
		Skus     []model.PackageSKU `json:"skus"`
		CoachIDs []int64            `json:"coachIds"`
	}
	list := make([]row, 0, len(pkgs))
	for _, p := range pkgs {
		r := row{CoursePackage: p, CoachIDs: []int64{}}
		if v, ok := skuMap[p.ID]; ok {
			r.Skus = v
		} else {
			r.Skus = []model.PackageSKU{}
		}
		if v, ok := coachMap[p.ID]; ok {
			r.CoachIDs = v
		}
		list = append(list, r)
	}
	resp.OK(c, gin.H{"list": list, "total": total})
}

// SavePackage POST（新增/更新合一：id>0 为更新）
func (h *AdminShopHandler) SavePackage(c *gin.Context) {
	var req packagePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	if req.Category != model.PkgCategoryPrivate && req.Category != model.PkgCategoryGroup && req.Category != model.PkgCategoryTrial {
		resp.Handle(c, errs.BadParam("分类必须是 private/group/trial"))
		return
	}
	// M6：价格/课时/有效期零与负数直接拒绝（0 元课包=免费送课入口，负价污染退款折算）
	// 新人价：启用时必须小于售价且 ≥1 分（每人限购一次，按手机号判重）
	for _, s := range req.Skus {
		if s.Price <= 0 || s.Sessions <= 0 || s.ValidDays <= 0 {
			resp.Handle(c, errs.BadParam("规格的价格、课时数、有效期必须大于 0"))
			return
		}
		if s.NewcomerPrice != 0 && (s.NewcomerPrice < 1 || s.NewcomerPrice >= s.Price) {
			resp.Handle(c, errs.BadParam("新人价需大于 0 且小于售价"))
			return
		}
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		var pkg model.CoursePackage
		isCreate := true
		if idStr := c.Param("id"); idStr != "" {
			id, _ := idParam(c, "id")
			if err := tx.First(&pkg, id).Error; err != nil {
				return errs.NotFound("课包不存在")
			}
			isCreate = false
		} else {
			pkg = model.CoursePackage{OwnerID: h.owner}
		}
		pkg.Title = req.Title
		pkg.CoverURL = req.CoverURL
		pkg.IntroVideoURL = req.IntroVideoURL
		pkg.Category = req.Category
		pkg.Tags = req.Tags
		pkg.Difficulty = req.Difficulty
		pkg.Outline = req.Outline
		pkg.Suitable = req.Suitable
		pkg.Status = req.Status
		pkg.Sort = req.Sort
		if err := tx.Save(&pkg).Error; err != nil {
			return err
		}
		// SKU：保留 payload 里带 id 的，新增 id=0 的，缺失的软删（M5：
		// 物理删除会让已支付订单入账查不到规格；已支付订单一律按下单快照入账）
		keep := map[int64]bool{}
		for _, s := range req.Skus {
			sku := model.PackageSKU{
				PackageID: pkg.ID, Name: s.Name, Sessions: s.Sessions, ValidDays: s.ValidDays,
				Price: s.Price, OriginalPrice: s.OriginalPrice, Status: s.Status, Sort: s.Sort,
			}
			if s.NewcomerPrice > 0 {
				np := s.NewcomerPrice
				sku.NewcomerPrice = &np
			}
			if s.ID > 0 {
				sku.ID = s.ID
				sku.CreatedAt = time.Now()
			}
			if err := tx.Save(&sku).Error; err != nil {
				return err
			}
			keep[sku.ID] = true // 新插入的 sku 此处已带回自增 ID
		}
		if len(keep) > 0 {
			if err := tx.Model(&model.PackageSKU{}).
				Where("package_id = ? AND id NOT IN ?", pkg.ID, keysOf(keep)).
				Update("status", 0).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Model(&model.PackageSKU{}).
				Where("package_id = ?", pkg.ID).Update("status", 0).Error; err != nil {
				return err
			}
		}
		// 教练关联全量替换
		if err := tx.Where("package_id = ?", pkg.ID).Delete(&model.CoachPackage{}).Error; err != nil {
			return err
		}
		for _, coachID := range req.CoachIDs {
			if err := tx.Create(&model.CoachPackage{CoachID: coachID, PackageID: pkg.ID}).Error; err != nil {
				return err
			}
		}
		_ = isCreate
		return nil
	})
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"saved": true})
}

func keysOf(m map[int64]bool) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	if len(out) == 0 {
		return []int64{0}
	}
	return out
}

// ---- 教练 ----

type coachPayload struct {
	Name         string `json:"name" binding:"required"`
	AvatarURL    string `json:"avatarUrl"`
	Gender       int8   `json:"gender"`
	Years        int    `json:"years"`
	Specialties  string `json:"specialties"`
	CertImages   string `json:"certImages"`
	Intro        string `json:"intro"`
	IntroVideo   string `json:"introVideoUrl"`
	UserID       int64  `json:"userId"`
	Status       int8   `json:"status"`
	Sort         int    `json:"sort"`
}

func (h *AdminShopHandler) ListCoaches(c *gin.Context) {
	page, size := pageOf(c)
	q := h.db.Model(&model.Coach{}).Where("owner_id = ?", h.owner)
	if kw := c.Query("keyword"); kw != "" {
		q = q.Where("name LIKE ?", "%"+kw+"%")
	}
	var total int64
	q.Count(&total)
	var coaches []model.Coach
	q.Order("status DESC, sort DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&coaches)
	resp.OK(c, gin.H{"list": coaches, "total": total})
}

func (h *AdminShopHandler) SaveCoach(c *gin.Context) {
	var req coachPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	var coach model.Coach
	if id, ok := idParam(c, "id"); ok {
		if err := h.db.First(&coach, id).Error; err != nil {
			resp.Handle(c, errs.NotFound("教练不存在"))
			return
		}
	} else {
		coach = model.Coach{OwnerID: h.owner}
	}
	coach.Name = req.Name
	coach.AvatarURL = req.AvatarURL
	coach.Gender = req.Gender
	coach.Years = req.Years
	coach.Specialties = req.Specialties
	coach.CertImages = req.CertImages
	coach.Intro = req.Intro
	coach.IntroVideo = req.IntroVideo
	if req.UserID > 0 {
		coach.UserID = req.UserID // 绑定小程序登录账号（双角色）
	}
	coach.Status = req.Status
	coach.Sort = req.Sort
	if err := h.db.Save(&coach).Error; err != nil {
		resp.Handle(c, errs.Server(err, "保存教练失败"))
		return
	}
	resp.OK(c, coach)
}

// VerifyCoachCert POST /admin/api/v1/coaches/:id/verify {verified: 1/0}
func (h *AdminShopHandler) VerifyCoachCert(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Verified int8 `json:"verified"`
	}
	_ = c.ShouldBindJSON(&req)
	res := h.db.Model(&model.Coach{}).Where("id = ?", id).Update("cert_verified", req.Verified)
	if res.Error != nil || res.RowsAffected == 0 {
		resp.Handle(c, errs.NotFound("教练不存在"))
		return
	}
	resp.OK(c, gin.H{"verified": req.Verified})
}

// ---- Banner ----

func (h *AdminShopHandler) ListBanners(c *gin.Context) {
	var banners []model.Banner
	h.db.Where("owner_id = ?", h.owner).Order("sort DESC, id DESC").Find(&banners)
	resp.OK(c, banners)
}

func (h *AdminShopHandler) SaveBanner(c *gin.Context) {
	var req model.Banner
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	req.OwnerID = h.owner
	if id, ok := idParam(c, "id"); ok && id > 0 {
		req.ID = id
	}
	if err := h.db.Save(&req).Error; err != nil {
		resp.Handle(c, errs.Server(err, "保存 Banner 失败"))
		return
	}
	resp.OK(c, req)
}

func (h *AdminShopHandler) DeleteBanner(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	h.db.Delete(&model.Banner{}, id)
	resp.OK(c, gin.H{"deleted": true})
}

// ---- 运营配置 ----

func (h *AdminShopHandler) ListConfigs(c *gin.Context) {
	rows, err := h.cfg.All(c.Request.Context())
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, rows)
}

func (h *AdminShopHandler) SaveConfig(c *gin.Context) {
	var req struct {
		Key    string `json:"key" binding:"required"`
		Value  string `json:"value"`
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Handle(c, service.BadParamJSON())
		return
	}
	// 只允许改已注册的配置键，防脏数据
	known := map[string]bool{
		model.CfgBookingCancelPenaltyHours: true, model.CfgGroupQuitDeadlineHours: true,
		model.CfgRefundFullDays: true, model.CfgMemberWarnSessions: true,
		model.CfgMemberWarnExpireDays: true, model.CfgReviewInviteAfterDays: true,
		model.CfgReviewAutoApprove: true, model.CfgNewcomerGiftTemplates: true,
		model.CfgPrivacyContent: true, model.CfgAgreementContent: true,
	}
	if !known[req.Key] {
		resp.Handle(c, errs.BadParam("不支持的配置项: "+req.Key))
		return
	}
	if err := h.cfg.Set(c.Request.Context(), req.Key, req.Value, req.Remark); err != nil {
		resp.Handle(c, errs.Server(err, "保存配置失败"))
		return
	}
	resp.OK(c, gin.H{"saved": true})
}
