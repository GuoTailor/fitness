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

	// 定时任务：评价邀请 / 课时与有效期预警（进程内每小时扫描）
	task.NewRunner(gdb, mp, cfg.Notify).Start(context.Background())

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: r,
	}
	log.Printf("fitness-server %s 监听 :%d（mode=%s wechat.mock=%v pay.mock=%v）",
		version, cfg.Server.Port, cfg.Server.Mode, cfg.WeChat.Mock, cfg.Pay.Mock)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务退出: %v", err)
	}
}
