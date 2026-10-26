package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fitness/internal/auth"
	"fitness/internal/db"
	"fitness/internal/model"
	"fitness/internal/sms"
	"fitness/internal/wechat"
)

func newSmsAuthSvc(t *testing.T) (*AuthService, *gorm.DB) {
	t.Helper()
	d, err := db.OpenSQLite()
	require.NoError(t, err)
	require.NoError(t, d.AutoMigrate(&model.User{}, &model.Coach{}, &model.SmsVerification{}, &model.SmsSendCounter{}))
	tokens := auth.NewManager("test-secret", 1)
	return NewAuthService(d, tokens, wechat.NewMock()).WithSms(sms.NewMock(), true), d
}

func backdateSms(t *testing.T, d *gorm.DB) {
	t.Helper()
	// 跳过 60s 重发间隔：只回拨最近发送时间，保留当日计数（日限由 counter 原子校验，P1-11）
	require.NoError(t, d.Model(&model.SmsSendCounter{}).Where("send_count > 0").
		Update("last_sent_at", time.Now().Add(-2*time.Minute)).Error)
}

func TestSendLoginSmsBasics(t *testing.T) {
	svc, _ := newSmsAuthSvc(t)
	ctx := context.Background()

	// 正常下发：6 位数字、mock 通道回显
	code, dev, err := svc.SendLoginSms(ctx, "13800138000")
	require.NoError(t, err)
	assert.True(t, dev)
	assert.Len(t, code, 6)
	for _, ch := range code {
		assert.True(t, ch >= '0' && ch <= '9')
	}

	// 60s 内重发被拒
	_, _, err = svc.SendLoginSms(ctx, "13800138000")
	assert.Error(t, err)

	// 非法手机号
	_, _, err = svc.SendLoginSms(ctx, "12345")
	assert.Error(t, err)
}

func TestSendLoginSmsDailyLimit(t *testing.T) {
	svc, d := newSmsAuthSvc(t)
	ctx := context.Background()

	for i := 0; i < smsDailyLimit; i++ {
		_, _, err := svc.SendLoginSms(ctx, "13800138000")
		require.NoError(t, err, "第 %d 条应发送成功", i+1)
		backdateSms(t, d) // 跳过 60s 重发间隔，保留当日计数
	}
	_, _, err := svc.SendLoginSms(ctx, "13800138000")
	assert.Error(t, err, "超过单日上限应被拒")
}

func TestLoginByPhoneFlow(t *testing.T) {
	svc, _ := newSmsAuthSvc(t)
	ctx := context.Background()

	code, _, err := svc.SendLoginSms(ctx, "13800138000")
	require.NoError(t, err)

	// 错误验证码不计入成功，且不核销
	_, err = svc.LoginByPhone(ctx, "13800138000", "000000")
	assert.Error(t, err)

	res, err := svc.LoginByPhone(ctx, "13800138000", code)
	require.NoError(t, err)
	assert.NotEmpty(t, res.Token)
	assert.Equal(t, "13800138000", res.User.PlainPhone())
	assert.Equal(t, "用户8000", res.User.Nickname)

	// 验证码用后作废（防重放）
	_, err = svc.LoginByPhone(ctx, "13800138000", code)
	assert.Error(t, err)

	claims, err := auth.NewManager("test-secret", 1).Parse(res.Token)
	require.NoError(t, err)
	assert.Equal(t, res.User.ID, claims.UserID)
	assert.Equal(t, auth.RoleUser, claims.Role)

	// 再次登录走新验证码 → 复用同一账号
	backdateSms(t, svc.db)
	code2, _, err := svc.SendLoginSms(ctx, "13800138000")
	require.NoError(t, err)
	res2, err := svc.LoginByPhone(ctx, "13800138000", code2)
	require.NoError(t, err)
	assert.Equal(t, res.User.ID, res2.User.ID)
}

func TestLoginByPhoneAttemptLimit(t *testing.T) {
	svc, _ := newSmsAuthSvc(t)
	ctx := context.Background()

	code, _, err := svc.SendLoginSms(ctx, "13800138000")
	require.NoError(t, err)

	for i := 0; i < smsMaxAttempts; i++ {
		_, err = svc.LoginByPhone(ctx, "13800138000", "999999")
		assert.Error(t, err, "第 %d 次错误尝试应失败", i+1)
	}
	// 错误次数达上限后，正确验证码也不再放行（防暴力猜码）
	_, err = svc.LoginByPhone(ctx, "13800138000", code)
	assert.Error(t, err)
}

func TestLoginByPhoneBindsExistingWechatUser(t *testing.T) {
	svc, _ := newSmsAuthSvc(t)
	ctx := context.Background()

	// 小程序登录 + 绑定手机号
	res, err := svc.UserLogin(ctx, "mock-user-9")
	require.NoError(t, err)
	_, err = svc.BindPhone(ctx, res.User.ID, "13900139000")
	require.NoError(t, err)

	// 同手机号在网页端登录 → 同一账号
	backdateSms(t, svc.db)
	code, _, err := svc.SendLoginSms(ctx, "13900139000")
	require.NoError(t, err)
	res2, err := svc.LoginByPhone(ctx, "13900139000", code)
	require.NoError(t, err)
	assert.Equal(t, res.User.ID, res2.User.ID, "同手机号应复用微信端账号")
}

// 全链路：网页端扫码通道下单支付（mock 直成功入账），非法通道被拒
func TestPrepayNativeChannel(t *testing.T) {
	svc, d := newOrderSvc(t)
	ctx := context.Background()
	sku := seedPkg(t, d, 10, 10000)

	order, err := svc.Create(ctx, 1, sku.ID, 0)
	require.NoError(t, err)
	res, err := svc.Prepay(ctx, order.ID, 1, "native")
	require.NoError(t, err)
	assert.True(t, res.MockPaid, "mock 通道 native 应直接支付成功")

	got, err := svc.MyOrder(ctx, order.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, model.OrderStatusPaid, got.Status)

	// 非法通道
	order2, err := svc.Create(ctx, 1, sku.ID, 0)
	require.NoError(t, err)
	_, err = svc.Prepay(ctx, order2.ID, 1, "alipay")
	assert.Error(t, err)

	// 他人订单查询 → 拒绝
	_, err = svc.MyOrder(ctx, order.ID, 2)
	assert.Error(t, err)
}
