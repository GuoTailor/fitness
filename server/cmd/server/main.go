// fitness-server 健身课包售卖小程序后端。
// 启动：fitness-server [-config configs/config.yaml] [-migrate-only]
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fitness/internal/auth"
	"fitness/internal/config"
	"fitness/internal/db"
	"fitness/internal/migrate"
	"fitness/internal/router"
	"fitness/internal/service"
	"fitness/internal/task"
	"fitness/internal/wechat"
	"fitness/migrations"
)

var version = "dev"

func main() {
	var (
		configPath  = flag.String("config", "configs/config.yaml", "配置文件路径")
		migrateOnly = flag.Bool("migrate-only", false, "仅执行数据库迁移后退出")
		showVer     = flag.Bool("version", false, "打印版本")
	)
	flag.Parse()
	if *showVer {
		fmt.Println("fitness-server", version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	// 上线门禁（L1/L2/L3/M12）：release 模式禁止弱 secret / mock 通道 / 默认管理员密码
	if err := cfg.ValidateForRelease(); err != nil {
		log.Fatalf("配置不满足上线要求: %v", err)
	}
	if cfg.DB.Password == "" {
		log.Fatal("数据库密码为空：请在 config.local.yaml 或环境变量 DB_PASSWORD 配置")
	}

	gdb, err := db.OpenMySQL(cfg.DB)
	if err != nil {
		log.Fatalf("%v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	n, err := migrate.Up(ctx, gdb, migrations.FS)
	if err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	log.Printf("数据库迁移完成（本次应用 %d 个脚本）", n)

	authSvc := service.NewAuthService(gdb, auth.NewManager(cfg.JWT.Secret, cfg.JWT.ExpireHours), nil)
	authSvc.BootstrapAdmin(os.Getenv("ADMIN_INIT_PASSWORD"))

	if *migrateOnly {
		return
	}

	var mp wechat.MiniProgram
	if cfg.WeChat.Mock {
		mp = wechat.NewMock()
		log.Printf("[wechat] mock 登录模式：code 即 openid（如 mock-user-1）")
	} else {
		mp = wechat.NewMiniProgram(cfg.WeChat.AppID, cfg.WeChat.Secret)
	}

	var pay wechat.PaymentProvider
	if cfg.Pay.Mock {
		pay = wechat.NewMockPay()
		log.Printf("[pay] mock 支付模式：prepay 即支付成功并自动入账")
	} else {
		// 真实微信支付 v3（wechatpay-go）：资质办结后填写商户配置即可启用，
		// 回调路由 /api/v1/pay/notify 已就绪（见上线 checklist B 节）
		pay, err = wechat.NewRealPay(cfg.Pay, cfg.WeChat.AppID)
		if err != nil {
			log.Fatalf("[pay] 初始化真实支付通道失败: %v", err)
		}
		log.Printf("[pay] 微信支付 v3 通道已启用（mch=%s）", cfg.Pay.MchID)
	}

	tokens := auth.NewManager(cfg.JWT.Secret, cfg.JWT.ExpireHours)
	r := router.Build(cfg, gdb, tokens, mp, pay)

	// 定时任务：消息触达 + 状态推进（关单/过期/结课/退款轮询），进程内每小时扫描
	taskCtx, taskCancel := context.WithCancel(context.Background())
	defer taskCancel()
	task.NewRunner(gdb, mp, cfg.Notify, pay).Start(taskCtx)

	// HTTP 基线（E1）：超时防慢连接拖垮进程；SIGINT/SIGTERM 优雅停机
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Printf("收到退出信号，开始优雅停机…")
		shutdownCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("停机超时: %v", err)
		}
		taskCancel()
		os.Exit(0)
	}()

	log.Printf("fitness-server %s 监听 :%d（mode=%s wechat.mock=%v pay.mock=%v）",
		version, cfg.Server.Port, cfg.Server.Mode, cfg.WeChat.Mock, cfg.Pay.Mock)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务退出: %v", err)
	}
}
