package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 全局配置。敏感项（数据库密码、JWT secret）优先从
// config.local.yaml（不入库）与环境变量读取，方便开发/生产切换。
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	DB       DBConfig       `yaml:"db"`
	JWT      JWTConfig      `yaml:"jwt"`
	WeChat   WeChatConfig   `yaml:"wechat"`
	Pay      PayConfig      `yaml:"pay"`
	Notify   NotifyConfig   `yaml:"notify"`
	Sms      SmsConfig      `yaml:"sms"`
	Video    VideoConfig    `yaml:"video"`
	Upload   UploadConfig   `yaml:"upload"`
	OwnerID  int64          `yaml:"owner_id"` // 平台化预留：单店自营阶段固定 1
	Version  string         `yaml:"-"`
	LoadedAt time.Time      `yaml:"-"`
}

type ServerConfig struct {
	Port    int    `yaml:"port"`
	BaseURL string `yaml:"base_url"` // 对外地址，拼支付回调等
	Mode    string `yaml:"mode"`     // debug / release
	// TrustedProxies 信任的反向代理网段（如 nginx 所在本机 ["127.0.0.1"]）。
	// 空 = 不信任任何代理头，ClientIP 一律取直连地址（P0-5：防 XFF 伪造绕过限流）
	TrustedProxies []string `yaml:"trusted_proxies"`
	// CORSOrigins 允许跨域的 Origin 白名单（admin-web 本地开发等），空 = 不放行跨域
	CORSOrigins []string `yaml:"cors_origins"`
}

type DBConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	Params   string `yaml:"params"`
}

func (d DBConfig) DSN() string {
	params := d.Params
	if params == "" {
		params = "charset=utf8mb4&parseTime=True&loc=Local"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s", d.Username, d.Password, d.Host, d.Port, d.Database, params)
}

type JWTConfig struct {
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expire_hours"`
}

type WeChatConfig struct {
	// Mock 为 true 时登录/手机号走本地模拟，无需真实小程序资质
	Mock   bool   `yaml:"mock"`
	AppID  string `yaml:"appid"`
	Secret string `yaml:"secret"`
}

type PayConfig struct {
	// Mock 为 true 时下单直接走「模拟支付成功」，无需商户号
	Mock          bool   `yaml:"mock"`
	MchID         string `yaml:"mch_id"`
	MchSerialNo   string `yaml:"mch_serial_no"`
	APIv3Key      string `yaml:"apiv3_key"`
	PrivateKey    string `yaml:"private_key_path"` // 商户私钥文件路径
	NotifyURL     string `yaml:"notify_url"`
	RefundNotify  string `yaml:"refund_notify_url"`
}

type NotifyConfig struct {
	// Mock 为 true 时不真正调用微信订阅消息，仅记录 notify_log
	Mock      bool               `yaml:"mock"`
	templates map[string]string  `yaml:"-"`
	Templates map[string]string  `yaml:"templates"` // scene -> 模板 ID
}

type SmsConfig struct {
	// Mock 为 true 时不真正发送短信，验证码仅记日志并由登录响应回显（联调用）
	Mock bool `yaml:"mock"`
}

type VideoConfig struct {
	// 课程介绍视频的静态播放地址前缀（nginx 静态目录）
	BaseURL string `yaml:"base_url"`
}

// UploadConfig 图片上传（评价图片等）。文件落本地磁盘，nginx /uploads/ 静态直出。
type UploadConfig struct {
	Dir     string `yaml:"dir"`      // 存储目录，默认 data/uploads
	BaseURL string `yaml:"base_url"` // 对外访问前缀，默认 http://localhost:8080/uploads
	MaxMB   int    `yaml:"max_mb"`   // 单图上限 MB，默认 2
}

// Load 按顺序合并：config.yaml -> config.local.yaml -> 环境变量（DB_PASSWORD / JWT_SECRET）
func Load(path string) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port: 8080, Mode: "debug", BaseURL: "http://localhost:8080",
			// 开发默认放行 vite dev server（admin-web:5173 / client H5:5174）；
			// release 部署在 nginx 后同源，无需跨域
			CORSOrigins: []string{
				"http://localhost:5173", "http://127.0.0.1:5173",
				"http://localhost:5174", "http://127.0.0.1:5174",
			},
		},
		DB:     DBConfig{Host: "10.96.158.189", Port: 3306, Username: "fitness", Database: "fitness"},
		JWT:    JWTConfig{Secret: "dev-insecure-secret", ExpireHours: 24 * 30},
		WeChat: WeChatConfig{Mock: true},
		Pay:    PayConfig{Mock: true},
		Notify: NotifyConfig{Mock: true},
		Sms:    SmsConfig{Mock: true},
		Video:  VideoConfig{BaseURL: "http://localhost:8080/videos"},
		Upload: UploadConfig{Dir: "data/uploads", BaseURL: "http://localhost:8080/uploads", MaxMB: 2},
		OwnerID: 1,
	}
	if err := loadYAML(path, cfg); err != nil {
		return nil, err
	}
	// 同目录 local 覆盖（gitignore，放密码等敏感项）
	local := filepath.Join(filepath.Dir(path), "config.local.yaml")
	if _, err := os.Stat(local); err == nil {
		if err := loadYAML(local, cfg); err != nil {
			return nil, err
		}
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.DB.Password = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if cfg.JWT.ExpireHours <= 0 {
		cfg.JWT.ExpireHours = 24 * 30
	}
	cfg.LoadedAt = time.Now()
	return cfg, nil
}

func loadYAML(path string, out *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	return nil
}

// defaultJWTSecret 弱默认值，release 模式禁止使用（L1）
const defaultJWTSecret = "dev-insecure-secret"

// ValidateForRelease 上线门禁（L1/L2/L3/M12）：release 模式下弱 secret、
// mock 登录/支付、默认管理员密码一律 fail-fast。开发模式不生效。
func (c *Config) ValidateForRelease() error {
	if c.Server.Mode != "release" {
		return nil
	}
	var problems []string
	if c.JWT.Secret == "" || c.JWT.Secret == defaultJWTSecret {
		problems = append(problems, "jwt.secret 不能为空或使用开发默认值")
	}
	if len(c.JWT.Secret) < 16 {
		problems = append(problems, "jwt.secret 至少 16 位")
	}
	if c.WeChat.Mock {
		problems = append(problems, "wechat.mock 必须关闭（mock 登录可冒充任意用户）")
	}
	if c.Pay.Mock {
		problems = append(problems, "pay.mock 必须关闭（mock 支付=免费入账）")
	}
	if c.Sms.Mock {
		problems = append(problems, "sms.mock 必须关闭（mock 短信可在任意手机号登录）")
	}
	if os.Getenv("ADMIN_INIT_PASSWORD") == "" {
		problems = append(problems, "必须通过环境变量 ADMIN_INIT_PASSWORD 提供管理员初始密码")
	}
	if len(problems) > 0 {
		return fmt.Errorf("release 模式配置检查未通过：%s", strings.Join(problems, "；"))
	}
	return nil
}
