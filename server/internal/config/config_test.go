package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultModeRelease 默认配置走安全档：配置文件未声明 mode 时为 release（P0-4）
func TestDefaultModeRelease(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(p, []byte("server:\n  port: 8080\n"), 0o600))
	cfg, err := Load(p)
	require.NoError(t, err)
	assert.Equal(t, "release", cfg.Server.Mode)
}

// TestValidateForReleaseBlocksMocks release 模式强制关闭全部 mock 通道（P0-4）
func TestValidateForReleaseBlocksMocks(t *testing.T) {
	cfg := &Config{Server: ServerConfig{Mode: "release"}, JWT: JWTConfig{Secret: "0123456789abcdef"}}
	cfg.WeChat.Mock = true
	cfg.Pay.Mock = true
	cfg.Sms.Mock = true
	err := cfg.ValidateForRelease()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wechat.mock")
	assert.Contains(t, err.Error(), "pay.mock")
	assert.Contains(t, err.Error(), "sms.mock")
}
