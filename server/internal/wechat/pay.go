package wechat

import (
	"context"
	"fmt"
)

// PrepayResult 下单参数。Mock 模式直接返回支付成功，无需前端发起支付。
type PrepayResult struct {
	MockPaid bool   `json:"mockPaid"` // true 时前端无需调用支付，订单已入账
	PrepayID string `json:"prepayId,omitempty"`
	CodeURL  string `json:"codeUrl,omitempty"` // Native 扫码支付二维码内容（channel=native 时返回）
	// 小程序拉起支付所需参数（真实模式返回，原样透传 wx.requestPayment）
	PayParams map[string]string `json:"payParams,omitempty"`
}

// PaymentProvider 微信支付 v3 能力。真实实现见 pay_v3.go（wechatpay-go），
// 资质办结后 pay.mock=false 即切换；P0 开发期以 MockProvider 打通全部业务流。
type PaymentProvider interface {
	// Prepay JSAPI 统一下单：outTradeNo=订单号、openid=付款学员、amount 分
	Prepay(ctx context.Context, outTradeNo string, openid string, amount int64, description string) (*PrepayResult, error)
	// PrepayNative Native 扫码统一下单（网页端）：返回 code_url 由前端渲染二维码，
	// 用户扫码后走同一支付回调入账链路
	PrepayNative(ctx context.Context, outTradeNo string, amount int64, description string) (*PrepayResult, error)
	// Refund 发起退款（mock 无操作恒成功；真实通道走 v3 退款 API）
	Refund(ctx context.Context, refundNo, outTradeNo string, refundAmount, totalAmount int64, reason string) error
	// QueryRefund 退款结果查询（真实模式用；mock 恒成功）
	QueryRefund(ctx context.Context, refundNo string) (string, error) // 返回退款单状态
}

// MockProvider 模拟支付：Prepay 直接成功，退款即时成功
type MockProvider struct{}

func NewMockPay() *MockProvider { return &MockProvider{} }

func (m *MockProvider) Prepay(_ context.Context, outTradeNo, _ string, _ int64, _ string) (*PrepayResult, error) {
	return &PrepayResult{MockPaid: true, PrepayID: fmt.Sprintf("mock-prepay-%s", outTradeNo)}, nil
}

func (m *MockProvider) PrepayNative(_ context.Context, outTradeNo string, _ int64, _ string) (*PrepayResult, error) {
	return &PrepayResult{MockPaid: true, PrepayID: fmt.Sprintf("mock-prepay-%s", outTradeNo)}, nil
}

func (m *MockProvider) Refund(context.Context, string, string, int64, int64, string) error {
	return nil
}

func (m *MockProvider) QueryRefund(_ context.Context, refundNo string) (string, error) {
	return "SUCCESS", nil
}
