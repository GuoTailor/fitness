// Package sms 短信发送抽象。Mock 通道仅记录日志（验证码由登录响应回显，便于无资质联调）；
// 服务商就绪后实现 Provider 接入（如阿里云/腾讯云短信）。
package sms

import (
	"context"
	"fmt"
	"log"
)

// Provider 短信发送能力
type Provider interface {
	// Send 发送验证码短信
	Send(ctx context.Context, phone, code string) error
}

// MockProvider 模拟短信：只打日志，不真实发送
type MockProvider struct{}

func NewMock() *MockProvider { return &MockProvider{} }

func (m *MockProvider) Send(_ context.Context, phone, code string) error {
	log.Printf("[sms] mock 发送验证码至 %s: %s", phone, code)
	return nil
}

// New 按配置选择通道。真实服务商接入前，release 门禁会拦截 sms.mock=false。
func New(mock bool) (Provider, error) {
	if mock {
		return NewMock(), nil
	}
	return nil, fmt.Errorf("sms.mock=false 需要接入真实短信服务商（尚未实现，见 internal/sms）")
}
