// Package router 路由装配。
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"fitness/internal/auth"
	"fitness/internal/config"
	"fitness/internal/handler"
	"fitness/internal/middleware"
	"fitness/internal/service"
	"fitness/internal/wechat"
)

func Build(cfg *config.Config, db *gorm.DB, tokens *auth.Manager, mp wechat.MiniProgram, pay wechat.PaymentProvider) (*gin.Engine, error) {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	// 上传目录初始化（图片落本地磁盘）
	uploadSvc, err := service.NewUploadService(cfg.Upload)
	if err != nil {
		return nil, err
	}
	r := gin.New()
	// P0-5：不信任任何代理头（空配置时 ClientIP 取直连地址，防 XFF 伪造绕过限流）；
	// nginx 反代部署时在 config.yaml server.trusted_proxies 里声明代理网段
	if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		gin.DefaultErrorWriter.Write([]byte("SetTrustedProxies: " + err.Error()))
	}
	// E1：请求体上限（multipart 上传单图 ≤2MB 由服务层限制，此处 10MB 防超大包拖垮服务）
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20)
		c.Next()
	})
	r.Use(gin.Logger(), gin.Recovery(), middleware.CORS(cfg.Server.CORSOrigins))
	// 上传文件静态直出（开发模式服务直出；生产建议 nginx alias 同一目录）
	r.Static("/uploads", uploadSvc.Dir())

	couponSvc := service.NewCouponService(db)
	authSvc := service.NewAuthService(db, tokens, mp).
		WithLockout(middleware.NewAdminLockout(5, 15)). // 连续失败5次锁15分钟
		WithCouponGifts(couponSvc)                     // 新用户自动发新人礼包
	cfgSvc := service.NewConfigService(db)
	shopSvc := service.NewShopService(db, cfg.OwnerID)
	orderSvc := service.NewOrderService(db, cfgSvc, pay)
	memberSvc := service.NewMemberService(db)
	notifySvc := service.NewNotifyService(db, mp, cfg.Notify.Mock, cfg.Notify.Templates)
	bookingSvc := service.NewBookingService(db, cfgSvc).WithNotify(notifySvc)
	groupSvc := service.NewGroupService(db, cfgSvc).WithNotify(notifySvc)
	verifySvc := service.NewVerifyService(db)
	reviewSvc := service.NewReviewService(db, cfgSvc).WithUploadPrefixes(cfg.Upload.BaseURL)
	statsSvc := service.NewStatsService(db)

	authH := handler.NewAuthHandler(authSvc)
	meH := handler.NewMeHandler(db)
	shopH := handler.NewShopHandler(shopSvc)
	orderH := handler.NewOrderHandler(orderSvc, memberSvc)
	bookingH := handler.NewBookingHandler(bookingSvc, groupSvc, verifySvc)
	coachH := handler.NewCoachHandler(db, bookingSvc, groupSvc, verifySvc, cfg.OwnerID)
	adminShopH := handler.NewAdminShopHandler(db, cfgSvc, cfg)
	adminOrderH := handler.NewAdminOrderHandler(db, orderSvc, memberSvc)
	adminBookingH := handler.NewAdminBookingHandler(db, bookingSvc, verifySvc)
	reviewH := handler.NewReviewHandler(reviewSvc, verifySvc, cfg.Notify)
	statsH := handler.NewStatsHandler(statsSvc)
	uploadH := handler.NewUploadHandler(uploadSvc)
	miscH := handler.NewMiscHandler(cfgSvc)
	couponH := handler.NewCouponHandler(couponSvc)
	adminCouponH := handler.NewAdminCouponHandler(couponSvc)

	r.GET("/healthz", handler.Ping)

	// 微信支付回调（公共路由：验签即认证，不走 JWT）
	payH := handler.NewPayHandler(orderSvc, pay)
	r.POST("/api/v1/pay/notify", payH.Notify)
	r.POST("/api/v1/pay/refund/notify", payH.RefundNotify)
	// 协议页（公开内容，后台可编辑）
	r.GET("/api/v1/agreement", miscH.Agreement)

	// ---- 学员端 / 教练端（同小程序）----
	api := r.Group("/api/v1")
	api.Use(middleware.JWT(tokens))
	{
		api.POST("/auth/login", middleware.LoginRateLimit(10), authH.Login)
		api.POST("/auth/phone", middleware.RequireUser(), authH.BindPhone)
		api.GET("/me", middleware.RequireUser(), meH.Me)

		api.GET("/home", shopH.Home)
		api.GET("/packages", shopH.ListPackages)
		api.GET("/packages/:id", shopH.PackageDetail)
		api.GET("/coaches", shopH.ListCoaches)
		api.GET("/coaches/:id", shopH.CoachDetail)

		api.POST("/orders", middleware.RequireUser(), orderH.Create)
		api.GET("/orders", middleware.RequireUser(), orderH.My)
		api.POST("/orders/:id/prepay", middleware.RequireUser(), orderH.Prepay)
		api.POST("/orders/:id/refund", middleware.RequireUser(), orderH.ApplyRefund)
		api.GET("/member/packages", middleware.RequireUser(), orderH.MyPackages)
		api.GET("/member/packages/:id/transactions", middleware.RequireUser(), orderH.MyTransactions)
		// 优惠券（领券中心 / 我的券包 / 可用券）
		api.GET("/coupons/center", middleware.RequireUser(), couponH.Center)
		api.GET("/coupons/mine", middleware.RequireUser(), couponH.Mine)
		api.GET("/coupons/usable", middleware.RequireUser(), couponH.Usable)
		api.POST("/coupons/:id/claim", middleware.RequireUser(), couponH.Claim)
		// 图片上传（评价图等）
		api.POST("/uploads", middleware.RequireUser(), uploadH.Create)

		// 私教预约
		api.GET("/coaches/:id/slots", bookingH.Slots)
		api.POST("/bookings", middleware.RequireUser(), bookingH.Create)
		api.GET("/bookings", middleware.RequireUser(), bookingH.My)
		api.POST("/bookings/:id/cancel", middleware.RequireUser(), bookingH.Cancel)
		api.GET("/bookings/:id/code", middleware.RequireUser(), bookingH.Code)
		// 团课
		api.GET("/group-classes", bookingH.GroupList)
		api.GET("/group-classes/:id", bookingH.GroupDetail)
		api.POST("/group-classes/:id/enroll", middleware.RequireUser(), bookingH.GroupEnroll)
		api.POST("/group-classes/:id/quit", middleware.RequireUser(), bookingH.GroupQuit)
		api.GET("/group-classes/:id/code", middleware.RequireUser(), bookingH.GroupCode)
		// 训练记录
		api.GET("/training", middleware.RequireUser(), bookingH.Training)
		api.GET("/training/calendar", middleware.RequireUser(), reviewH.TrainingCalendar)
		// 评价
		api.GET("/reviews/reviewable", middleware.RequireUser(), reviewH.Reviewable)
		api.POST("/reviews", middleware.RequireUser(), reviewH.Submit)
		api.GET("/reviews/mine", middleware.RequireUser(), reviewH.Mine)
		api.GET("/packages/:id/reviews", reviewH.PackageReviews)
		api.GET("/coaches/:id/reviews", reviewH.CoachReviewsPublic)
		// 订阅消息模板（mock 时为空，客户端跳过授权弹窗）
		api.GET("/notify/templates", reviewH.NotifyTemplates)
	}

	// ---- 教练工作台（双角色）----
	coach := api.Group("/coach", middleware.RequireCoach(db))
	{
		coach.GET("/profile", coachH.GetProfile)
		coach.PUT("/profile", coachH.UpdateProfile)
		coach.GET("/packages", coachH.MyPackages)
		coach.GET("/schedule", coachH.Schedule)
		coach.PUT("/schedule", coachH.SaveSchedule)
		coach.GET("/bookings", coachH.Day)
		coach.POST("/bookings/:id/confirm", coachH.Confirm)
		coach.POST("/bookings/:id/reject", coachH.RejectBooking)
		coach.GET("/group-classes", coachH.MyGroups)
		coach.POST("/group-classes", coachH.PublishGroup)
		coach.GET("/group-classes/:id/enrollments", coachH.Enrollments)
		coach.POST("/group-classes/:id/cancel", coachH.CancelGroup)
		coach.POST("/verify", coachH.Verify)
		coach.GET("/stats", coachH.Stats)
		coach.GET("/reviews", reviewH.CoachMyReviews)
		coach.POST("/reviews/:id/reply", reviewH.CoachReply)
	}

	// ---- 管理后台 ----
	admin := r.Group("/admin/api/v1")
	admin.Use(middleware.JWT(tokens))
	{
		admin.POST("/auth/login", middleware.LoginRateLimit(10), authH.AdminLogin)
	}
	a := admin.Group("", middleware.RequireAdmin())
	{
		// 数据看板
		a.GET("/dashboard", statsH.Dashboard)
		// 课包
		a.GET("/packages", adminShopH.ListPackages)
		a.POST("/packages", adminShopH.SavePackage)
		a.POST("/packages/:id", adminShopH.SavePackage)
		// 教练
		a.GET("/coaches", adminShopH.ListCoaches)
		a.POST("/coaches", adminShopH.SaveCoach)
		a.POST("/coaches/:id", adminShopH.SaveCoach)
		a.POST("/coaches/:id/verify", adminShopH.VerifyCoachCert)
		// Banner
		a.GET("/banners", adminShopH.ListBanners)
		a.POST("/banners", adminShopH.SaveBanner)
		a.POST("/banners/:id", adminShopH.SaveBanner)
		a.DELETE("/banners/:id", adminShopH.DeleteBanner)
		// 订单与退款（审批=真实打款，仅老板）
		a.GET("/orders", adminOrderH.ListOrders)
		a.GET("/orders/:id", adminOrderH.GetOrder)
		a.GET("/refunds", adminOrderH.ListRefunds)
		a.POST("/refunds/:id/approve", middleware.RequireBoss(), adminOrderH.ApproveRefund)
		a.POST("/refunds/:id/reject", middleware.RequireBoss(), adminOrderH.RejectRefund)
		// 学员（手动调整课时=资金类操作，仅老板）
		a.GET("/members", adminOrderH.ListMembers)
		a.GET("/members/:id/packages", adminOrderH.MemberPackages)
		a.POST("/members/:id/packages/:mpId/adjust", middleware.RequireBoss(), adminOrderH.AdjustSessions)
		// 预约课表与代客操作
		a.GET("/schedule", adminBookingH.ScheduleView)
		a.POST("/bookings", adminBookingH.CreateForUser)
		a.POST("/bookings/:id/cancel", adminBookingH.Cancel)
		a.POST("/verify", adminBookingH.Verify)
		// 运营配置（规则类，仅老板；查看全员）
		a.GET("/configs", adminShopH.ListConfigs)
		a.POST("/configs", middleware.RequireBoss(), adminShopH.SaveConfig)
		// 优惠券（建券/停用/定向发=资金类操作，仅老板；列表查看全员）
		a.GET("/coupons", adminCouponH.List)
		a.POST("/coupons", middleware.RequireBoss(), adminCouponH.Save)
		a.POST("/coupons/:id", middleware.RequireBoss(), adminCouponH.Save)
		a.POST("/coupons/:id/status", middleware.RequireBoss(), adminCouponH.Toggle)
		a.POST("/coupons/:id/grant", middleware.RequireBoss(), adminCouponH.Grant)
		// 评价管理
		a.GET("/reviews", reviewH.AdminList)
		a.POST("/reviews/:id/audit", reviewH.AdminAudit)
		a.POST("/reviews/:id/featured", reviewH.AdminFeatured)
	}

	return r, nil
}
