// Package wechat 微信能力封装。所有外部依赖（code2session、手机号、支付、订阅消息）
// 都收敛在此包，并用 mock 开关保证无资质时全流程可开发联调。
package wechat

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// LoginSession code2session 结果
type LoginSession struct {
	OpenID     string
	UnionID    string
	SessionKey string
}

// MiniProgram 小程序登录/手机号/订阅消息能力
type MiniProgram interface {
	Code2Session(ctx context.Context, code string) (*LoginSession, error)
	// GetPhoneNumber 手机号快速验证组件的新版 phone-code 接口
	GetPhoneNumber(ctx context.Context, phoneCode string) (string, error)
	// SendSubscribe 订阅消息推送。data 的 key 为模板字段名（如 thing1/date2），值为字段内容
	SendSubscribe(ctx context.Context, openid, templateID string, data map[string]string) error
}

// ---- 真实实现 ----

type miniApp struct {
	appid, secret string
	client        *http.Client

	tokMu      sync.Mutex
	token      string
	tokenUntil time.Time
}

func NewMiniProgram(appid, secret string) MiniProgram {
	return &miniApp{appid: appid, secret: secret, client: &http.Client{Timeout: 10 * time.Second}}
}

// accessToken 接口调用凭证（带缓存）：官方有效期 7200s，本地提前 5 分钟过期。
// 此前每次发送/取手机号都现取一次，触发微信接口频控。
func (m *miniApp) accessToken(ctx context.Context) (string, error) {
	m.tokMu.Lock()
	defer m.tokMu.Unlock()
	if m.token != "" && time.Now().Before(m.tokenUntil) {
		return m.token, nil
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := m.get(ctx, "/cgi-bin/token", url.Values{"grant_type": {"client_credential"}}, &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", errors.New("获取 access_token 失败")
	}
	m.token = out.AccessToken
	m.tokenUntil = time.Now().Add(115 * time.Minute)
	return m.token, nil
}

func (m *miniApp) get(ctx context.Context, path string, params url.Values, out interface{}) error {
	params.Set("appid", m.appid)
	params.Set("secret", m.secret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.weixin.qq.com"+path+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var base struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &base); err != nil {
		return fmt.Errorf("解析微信响应失败: %s", truncate(string(body), 200))
	}
	if base.ErrCode != 0 {
		return fmt.Errorf("微信接口错误 %d: %s", base.ErrCode, base.ErrMsg)
	}
	return json.Unmarshal(body, out)
}

func (m *miniApp) Code2Session(ctx context.Context, code string) (*LoginSession, error) {
	var out struct {
		OpenID     string `json:"openid"`
		UnionID    string `json:"unionid"`
		SessionKey string `json:"session_key"`
	}
	if err := m.get(ctx, "/sns/jscode2session", url.Values{"js_code": {code}, "grant_type": {"authorization_code"}}, &out); err != nil {
		return nil, err
	}
	return &LoginSession{OpenID: out.OpenID, UnionID: out.UnionID, SessionKey: out.SessionKey}, nil
}

func (m *miniApp) GetPhoneNumber(ctx context.Context, phoneCode string) (string, error) {
	tokenStr, err := m.accessToken(ctx)
	if err != nil {
		return "", err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token="+tokenStr,
		strings.NewReader(fmt.Sprintf(`{"code":%q}`, phoneCode)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		ErrCode int `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		PhoneInfo struct {
			PurePhoneNumber string `json:"purePhoneNumber"`
		} `json:"phone_info"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("解析手机号响应失败: %s", truncate(string(body), 200))
	}
	if out.ErrCode != 0 {
		return "", fmt.Errorf("获取手机号失败 %d: %s", out.ErrCode, out.ErrMsg)
	}
	return out.PhoneInfo.PurePhoneNumber, nil
}

func (m *miniApp) SendSubscribe(ctx context.Context, openid, templateID string, data map[string]string) error {
	tokenStr, err := m.accessToken(ctx)
	if err != nil {
		return err
	}
	payload := map[string]interface{}{
		"touser":      openid,
		"template_id": templateID,
		"page":        "pages/index/index",
		"data":        map[string]interface{}{},
	}
	fields := payload["data"].(map[string]interface{})
	for k, v := range data {
		fields[k] = map[string]string{"value": v}
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token="+tokenStr,
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var out struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return fmt.Errorf("解析订阅消息响应失败: %s", truncate(string(respBody), 200))
	}
	if out.ErrCode != 0 {
		return fmt.Errorf("订阅消息发送失败 %d: %s", out.ErrCode, out.ErrMsg)
	}
	return nil
}

// ---- Mock 实现 ----

// MockMiniProgram 开发联调用：code 即 openid（可传 mock-user-1 等造指定账号），
// phone_code 即手机号明文。
type MockMiniProgram struct {
	mu sync.Mutex
	// PhoneDecrypt 演示用 session_key 解密旧版手机号数据的入口，P0 不需要
	sessionCache map[string]string
}

func NewMock() *MockMiniProgram { return &MockMiniProgram{sessionCache: map[string]string{}} }

func (m *MockMiniProgram) Code2Session(_ context.Context, code string) (*LoginSession, error) {
	if code == "" {
		return nil, errors.New("mock: code 不能为空")
	}
	openid := "mock-openid-" + code
	m.mu.Lock()
	m.sessionCache[openid] = "mock-session-key"
	m.mu.Unlock()
	return &LoginSession{OpenID: openid, SessionKey: "mock-session-key"}, nil
}

func (m *MockMiniProgram) GetPhoneNumber(_ context.Context, phoneCode string) (string, error) {
	if phoneCode == "" {
		return "", errors.New("mock: phoneCode 不能为空")
	}
	if strings.HasPrefix(phoneCode, "mock-phone-") {
		return strings.TrimPrefix(phoneCode, "mock-phone-"), nil
	}
	// 形如手机号本身也接受
	return phoneCode, nil
}

// SendSubscribe mock：不真正发送，仅记日志（notify_log 由 NotifyService 落库）
func (m *MockMiniProgram) SendSubscribe(_ context.Context, openid, templateID string, _ map[string]string) error {
	if openid == "" {
		return errors.New("mock: openid 不能为空")
	}
	if templateID == "" {
		return errors.New("mock: 模板 ID 未配置")
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// DecryptPhoneData 旧版 encryptedData 解密（当前未用，保留给无 phone-code 场景）
func DecryptPhoneData(sessionKey, encryptedData, iv string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(sessionKey)
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", err
	}
	ivB, err := base64.StdEncoding.DecodeString(iv)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	mode := cipher.NewCBCDecrypter(block, ivB)
	plain := make([]byte, len(data))
	mode.CryptBlocks(plain, data)
	// 去 PKCS#7 padding
	pad := int(plain[len(plain)-1])
	if pad > len(plain) || pad == 0 {
		return "", errors.New("bad padding")
	}
	plain = plain[:len(plain)-pad]
	var out struct {
		PurePhoneNumber string `json:"purePhoneNumber"`
	}
	if err := json.Unmarshal(plain, &out); err != nil {
		return "", err
	}
	return out.PurePhoneNumber, nil
}
