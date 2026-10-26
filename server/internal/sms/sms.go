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

// DisabledProvider 未接入真实服务商时的占位通道：发送一律失败，
// 保证 release 模式可正常启动，仅手机号登录不可用（其余功能不受影响）。
type DisabledProvider struct{}

func (d *DisabledProvider) Send(_ context.Context, phone, _ string) error {
	log.Printf("[sms] 未接入短信服务商，拒绝发送至 %s", phone)
	return fmt.Errorf("短信通道未启用（未接入服务商）")
}

// New 按配置选择通道。mock=true 为联调通道；mock=false 且未接入服务商时返回禁用通道。
func New(mock bool) (Provider, error) {
	if mock {
		return NewMock(), nil
	}
	return &DisabledProvider{}, nil
}
