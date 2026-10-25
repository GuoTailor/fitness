// 微信支付 v3 真实通道（wechatpay-go）。商户资质办结后：pay.mock=false 即启用；
// 开发期无法联调真实微信侧，本文件保证编译期就绪 + 业务流仍走 mock 全覆盖。
package wechat

import (
	"context"
	"fmt"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/services/refunddomestic"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"

	"fitness/internal/config"
)

// RealPay 微信支付 v3（JSAPI/小程序）
type RealPay struct {
	appid, mchID string
	cfg          config.PayConfig
	client       *core.Client
}

// NewRealPay 加载商户私钥并初始化客户端（自动下载平台证书、验签器）。
func NewRealPay(cfg config.PayConfig, appid string) (*RealPay, error) {
	if cfg.MchID == "" || cfg.APIv3Key == "" || cfg.PrivateKey == "" {
		return nil, fmt.Errorf("pay 配置不完整：需要 mch_id / apiv3_key / private_key_path")
	}
	key, err := utils.LoadPrivateKeyWithPath(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("加载商户私钥失败: %w", err)
	}
	client, err := core.NewClient(context.Background(),
		option.WithWechatPayAutoAuthCipher(cfg.MchID, cfg.MchSerialNo, key, cfg.APIv3Key))
	if err != nil {
		return nil, fmt.Errorf("初始化微信支付客户端失败: %w", err)
	}
	return &RealPay{appid: appid, mchID: cfg.MchID, cfg: cfg, client: client}, nil
}

// Prepay 统一下单并返回小程序 wx.requestPayment 所需全部参数
func (p *RealPay) Prepay(ctx context.Context, outTradeNo, openid string, amount int64, description string) (*PrepayResult, error) {
	svc := jsapi.JsapiApiService{Client: p.client}
	resp, _, err := svc.PrepayWithRequestPayment(ctx, jsapi.PrepayRequest{
		Appid:       core.String(p.appid),
		Mchid:       core.String(p.mchID),
		Description: core.String(description),
		OutTradeNo:  core.String(outTradeNo),
		NotifyUrl:   core.String(p.cfg.NotifyURL),
		Amount:      &jsapi.Amount{Total: core.Int64(amount), Currency: core.String("CNY")},
		Payer:       &jsapi.Payer{Openid: core.String(openid)},
	})
	if err != nil {
		return nil, err
	}
	return &PrepayResult{
		PrepayID: deref(resp.PrepayId),
		PayParams: map[string]string{
			"timeStamp": deref(resp.TimeStamp),
			"nonceStr":  deref(resp.NonceStr),
			"package":   deref(resp.Package),
			"signType":  deref(resp.SignType),
			"paySign":   deref(resp.PaySign),
		},
	}, nil
}

// PrepayNative Native 扫码统一下单（网页端）：返回 weixin:// 二维码内容，
// 用户扫码支付后走与 JSAPI 相同的回调入账链路（/api/v1/pay/notify）。
func (p *RealPay) PrepayNative(ctx context.Context, outTradeNo string, amount int64, description string) (*PrepayResult, error) {
	svc := native.NativeApiService{Client: p.client}
	resp, _, err := svc.Prepay(ctx, native.PrepayRequest{
		Appid:       core.String(p.appid),
		Mchid:       core.String(p.mchID),
		Description: core.String(description),
		OutTradeNo:  core.String(outTradeNo),
		NotifyUrl:   core.String(p.cfg.NotifyURL),
		Amount:      &native.Amount{Total: core.Int64(amount), Currency: core.String("CNY")},
	})
	if err != nil {
		return nil, err
	}
	return &PrepayResult{CodeURL: deref(resp.CodeUrl)}, nil
}

// Refund 发起退款（out_refund_no 幂等：同号重复请求微信侧只退一笔）
func (p *RealPay) Refund(ctx context.Context, refundNo, outTradeNo string, refundAmount, totalAmount int64, reason string) error {
	svc := refunddomestic.RefundsApiService{Client: p.client}
	_, _, err := svc.Create(ctx, refunddomestic.CreateRequest{
		OutTradeNo: core.String(outTradeNo),
		OutRefundNo: core.String(refundNo),
		Reason:     core.String(reason),
		NotifyUrl:  core.String(p.cfg.RefundNotify),
		Amount: &refunddomestic.AmountReq{
			Refund: core.Int64(refundAmount), Total: core.Int64(totalAmount), Currency: core.String("CNY"),
		},
	})
	return err
}

// QueryRefund 按商户退款单号查退款状态（SUCCESS/PROCESSING/ABNORMAL/CLOSED）
func (p *RealPay) QueryRefund(ctx context.Context, refundNo string) (string, error) {
	svc := refunddomestic.RefundsApiService{Client: p.client}
	resp, _, err := svc.QueryByOutRefundNo(ctx, refunddomestic.QueryByOutRefundNoRequest{OutRefundNo: core.String(refundNo)})
	if err != nil {
		return "", err
	}
	if resp.Status == nil {
		return "", nil
	}
	return string(*resp.Status), nil
}

// NotifyHandler 支付回调验签处理器（平台证书自动管理注册在 downloader 单例中）
func (p *RealPay) NotifyHandler() (*notify.Handler, error) {
	visitor := downloader.MgrInstance().GetCertificateVisitor(p.mchID)
	if visitor == nil {
		return nil, fmt.Errorf("平台证书尚未就绪，请稍后重试")
	}
	return notify.NewNotifyHandler(p.cfg.APIv3Key, verifiers.NewSHA256WithRSAVerifier(visitor)), nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
