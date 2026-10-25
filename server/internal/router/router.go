// Package router 路由装配。
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"fitness/internal/auth"
	"fitness/internal/config"
	"fitness/internal/errs"
	"fitness/internal/handler"
	"fitness/internal/middleware"
	"fitness/internal/resp"
	"fitness/internal/service"
	"fitness/internal/task"
	"fitness/internal/wechat"
)

func Build(cfg *config.Config, db *gorm.DB, tokens *auth.Manager, mp wechat.MiniProgram, pay wechat.PaymentProvider, tasks *task.Runner) (*gin.Engine, error) {
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
	cfgSvc := service.NewConfigService(db)
	notifySvc := service.NewNotifyService(db, mp, cfg.Notify.Mock, cfg.Notify.Templates)
	referralSvc := service.NewReferralService(db, couponSvc, cfgSvc)
	checkinSvc := service.NewCheckinService(db, couponSvc, cfgSvc)
	gbSvc := service.NewGroupBuyService(db, cfgSvc).WithNotify(notifySvc)
	authSvc := service.NewAuthService(db, tokens, mp).
		WithLockout(middleware.NewAdminLockout(5, 15)). // 连续失败5次锁15分钟
		WithCouponGifts(couponSvc)                      // 新用户自动发新人礼包
	shopSvc := service.NewShopService(db, cfg.OwnerID)
	orderSvc := service.NewOrderService(db, cfgSvc, pay).WithReferralReward(referralSvc).WithGroupBuy(gbSvc)
	memberSvc := service.NewMemberService(db)
	bookingSvc := service.NewBookingService(db, cfgSvc).WithNotify(notifySvc)
	groupSvc := service.NewGroupService(db, cfgSvc).WithNotify(notifySvc)
	verifySvc := service.NewVerifyService(db)
	feedbackSvc := service.NewFeedbackService(db).WithNotify(notifySvc)
	bodySvc := service.NewBodyService(db)
	qaSvc := service.NewQaService(db).WithNotify(notifySvc)
	coachApplySvc := service.NewCoachApplicationService(db).WithNotify(notifySvc).WithUploadPrefixes(cfg.Upload.BaseURL)
	auditSvc := service.NewAuditService(db)
	reviewSvc := service.NewReviewService(db, cfgSvc).WithUploadPrefixes(cfg.Upload.BaseURL).WithRewardCoupons(couponSvc)
	statsSvc := service.NewStatsService(db)

	authH := handler.NewAuthHandler(authSvc)
	meH := handler.NewMeHandler(db)
	shopH := handler.NewShopHandler(shopSvc)
	orderH := handler.NewOrderHandler(orderSvc, memberSvc)
	bookingH := handler.NewBookingHandler(bookingSvc, groupSvc, verifySvc, feedbackSvc)
	coachH := handler.NewCoachHandler(db, bookingSvc, groupSvc, verifySvc, cfg.OwnerID)
	adminShopH := handler.NewAdminShopHandler(db, cfgSvc, cfg).WithAudit(auditSvc)
	adminOrderH := handler.NewAdminOrderHandler(db, orderSvc, memberSvc).WithAudit(auditSvc)
	adminBookingH := handler.NewAdminBookingHandler(db, bookingSvc, verifySvc).WithAudit(auditSvc)
	reviewH := handler.NewReviewHandler(reviewSvc, verifySvc, cfg.Notify).WithAudit(auditSvc)
	statsH := handler.NewStatsHandler(statsSvc)
	uploadH := handler.NewUploadHandler(uploadSvc)
	miscH := handler.NewMiscHandler(cfgSvc)
	couponH := handler.NewCouponHandler(couponSvc)
	adminCouponH := handler.NewAdminCouponHandler(couponSvc).WithAudit(auditSvc)
	feedbackH := handler.NewFeedbackHandler(feedbackSvc, bodySvc, verifySvc)
	qaH := handler.NewQaHandler(qaSvc)
	adminAuditH := handler.NewAdminAuditHandler(auditSvc)
	referralH := handler.NewReferralHandler(referralSvc)
	checkinH := handler.NewCheckinHandler(checkinSvc)
	gbH := handler.NewGroupBuyHandler(gbSvc)
	adminGBActH := handler.NewAdminGroupBuyActivityHandler(gbSvc).WithAudit(auditSvc)
	coachApplyH := handler.NewCoachApplyHandler(coachApplySvc).WithAudit(auditSvc)

	r.GET("/healthz", handler.Ping)

	// 微信支付回调（公共路由：验签即认证，不走 JWT）
	payH := handler.NewPayHandler(orderSvc, pay)
	r.POST("/api/v1/pay/notify", payH.Notify)
	r.POST("/api/v1/pay/refund/notify", payH.RefundNotify)
	// 协议页（公开内容，后台可编辑）
	r.GET("/api/v1/agreement", miscH.Agreement)

	// ---- 学员端 / 教练端（同小程序）----
	api := r.Group("/api/v1")
	api.Use(middleware.JWT(tokens), middleware.StatusGate(db))
	{
		api.POST("/auth/login", middleware.LoginRateLimit(10), authH.Login)
		api.POST("/auth/logout", middleware.RequireUser(), authH.Logout)
		api.POST("/auth/phone", middleware.RequireUser(), authH.BindPhone)
		api.GET("/me", middleware.RequireUser(), meH.Me)
		api.PUT("/me", middleware.RequireUser(), meH.UpdateMe)

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
		// 老带新裂变（邀请码即用户 ID，首单实付后双方发券）
		api.POST("/referral/bind", middleware.RequireUser(), referralH.Bind)
		api.GET("/referral/mine", middleware.RequireUser(), referralH.Mine)
		// 打卡挑战（月度上课达标发券）
		api.GET("/checkin/challenge", middleware.RequireUser(), checkinH.Status)
		api.POST("/checkin/claim", middleware.RequireUser(), checkinH.Claim)
		// 拼团（开团/参团返回待支付订单，再走 prepay）
		api.POST("/group-buys", middleware.RequireUser(), gbH.Open)
		api.GET("/group-buys", middleware.RequireUser(), gbH.ListOpen)
		api.GET("/group-buys/mine", middleware.RequireUser(), gbH.My)
		api.GET("/group-buys/:id", middleware.RequireUser(), gbH.Detail)
		api.POST("/group-buys/:id/join", middleware.RequireUser(), gbH.Join)
		api.GET("/group-buy-activities/:id", gbH.ActivityInfo)
		// 教练入驻申请
		api.POST("/coach-applications", middleware.RequireUser(), coachApplyH.Submit)
		api.GET("/coach-applications/mine", middleware.RequireUser(), coachApplyH.Mine)
		// 图片上传（评价图等）
		api.POST("/uploads", middleware.RequireUser(), uploadH.Create)

		// 私教预约
		api.GET("/coaches/:id/slots", bookingH.Slots)
		api.POST("/bookings", middleware.RequireUser(), bookingH.Create)
		api.GET("/bookings", middleware.RequireUser(), bookingH.My)
		api.GET("/bookings/:id", middleware.RequireUser(), bookingH.Detail)
		api.POST("/bookings/:id/cancel", middleware.RequireUser(), bookingH.Cancel)
		api.GET("/bookings/:id/code", middleware.RequireUser(), bookingH.Code)
		// 团课
		api.GET("/group-classes", bookingH.GroupList)
		api.GET("/group-classes/:id", bookingH.GroupDetail)
		api.POST("/group-classes/:id/enroll", middleware.RequireUser(), bookingH.GroupEnroll)
		api.POST("/group-classes/:id/quit", middleware.RequireUser(), bookingH.GroupQuit)
		api.POST("/group-classes/:id/waitlist", middleware.RequireUser(), bookingH.GroupWaitlist)
		api.POST("/group-classes/:id/waitlist/cancel", middleware.RequireUser(), bookingH.GroupWaitlistCancel)
		api.GET("/group-classes/:id/code", middleware.RequireUser(), bookingH.GroupCode)
		// 训练记录
		api.GET("/training", middleware.RequireUser(), bookingH.Training)
		api.GET("/training/calendar", middleware.RequireUser(), reviewH.TrainingCalendar)
		api.GET("/training/report", middleware.RequireUser(), feedbackH.Report)
		// 课后反馈与体重
		api.GET("/feedback/mine", middleware.RequireUser(), feedbackH.Mine)
		api.POST("/body-metrics", middleware.RequireUser(), feedbackH.AddMetric)
		api.GET("/body-metrics", middleware.RequireUser(), feedbackH.ListMetrics)
		// 课程问答（公开列表；提问/回复需登录）
		api.GET("/packages/:id/qa", qaH.ListOfPackage)
		api.POST("/qa/questions", middleware.RequireUser(), qaH.Ask)
		api.GET("/qa/mine", middleware.RequireUser(), qaH.MyQuestions)
		api.POST("/qa/questions/:id/answers", middleware.RequireUser(), qaH.Answer)
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
		// 教练请假（当日时段屏蔽）
		coach.GET("/leaves", coachH.Leaves)
		coach.POST("/leaves", coachH.AddLeave)
		coach.DELETE("/leaves/:id", coachH.CancelLeave)
		coach.GET("/bookings", coachH.Day)
		coach.POST("/bookings/:id/confirm", coachH.Confirm)
		coach.POST("/bookings/:id/reject", coachH.RejectBooking)
		coach.GET("/group-classes", coachH.MyGroups)
		coach.POST("/group-classes", coachH.PublishGroup)
		coach.GET("/group-classes/:id/enrollments", coachH.Enrollments)
		coach.POST("/group-classes/:id/cancel", coachH.CancelGroup)
		coach.POST("/verify", coachH.Verify)
		// 课后反馈（学员记录）
		coach.POST("/feedback", feedbackH.Write)
		coach.GET("/feedbacks", feedbackH.CoachList)
		// 留言答疑
		coach.GET("/qa", qaH.CoachQuestions)
		coach.POST("/qa/:id/answers", qaH.CoachAnswer)
		coach.POST("/qa/:id/pin", qaH.CoachPin)
		coach.GET("/stats", coachH.Stats)
		coach.GET("/reviews", reviewH.CoachMyReviews)
		coach.POST("/reviews/:id/reply", reviewH.CoachReply)
	}

	// ---- 管理后台 ----
	admin := r.Group("/admin/api/v1")
	admin.Use(middleware.JWT(tokens), middleware.StatusGate(db))
	{
		admin.POST("/auth/login", middleware.LoginRateLimit(10), authH.AdminLogin)
		admin.POST("/auth/logout", authH.AdminLogout)
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
		// 问答区管理（先发后审：隐藏违规、置顶沉淀 FAQ）
		a.GET("/qa", qaH.AdminList)
		a.POST("/qa/questions/:id/status", qaH.SetQuestionStatus)
		a.POST("/qa/questions/:id/pin", qaH.AdminPin)
		a.POST("/qa/answers/:id/status", qaH.SetAnswerStatus)
		// 操作审计日志（仅老板）
		a.GET("/audit-logs", middleware.RequireBoss(), adminAuditH.List)
		// 老带新邀请记录
		a.GET("/referrals", referralH.AdminList)
		// 拼团列表
		a.GET("/group-buys", gbH.AdminList)
		// 拼团限时活动（查询开放；增删改仅老板）
		a.GET("/group-buy-activities", adminGBActH.List)
		a.POST("/group-buy-activities", middleware.RequireBoss(), adminGBActH.Save)
		a.POST("/group-buy-activities/:id", middleware.RequireBoss(), adminGBActH.Save)
		a.DELETE("/group-buy-activities/:id", middleware.RequireBoss(), adminGBActH.Delete)
		// 教练入驻审核（敏感操作，仅老板）
		a.GET("/coach-applications", coachApplyH.AdminList)
		a.POST("/coach-applications/:id/approve", middleware.RequireBoss(), coachApplyH.Approve)
		a.POST("/coach-applications/:id/reject", middleware.RequireBoss(), coachApplyH.Reject)
		// 手动触发一轮定时任务（仅 mock 环境、仅老板）——冒烟/E2E 验证任务链路用
		a.POST("/tasks/run", func(c *gin.Context) {
			if !cfg.WeChat.Mock || !cfg.Pay.Mock || !cfg.Notify.Mock {
				resp.Handle(c, errs.Forbidden("仅开发环境可手动触发任务"))
				return
			}
			tasks.RunOnce(c.Request.Context())
			resp.OK(c, gin.H{"ran": true})
		})
	}

	return r, nil
}
