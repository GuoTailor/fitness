package service

// P1 二期 · 批次⑤ 单测：操作审计日志 + 资料编辑

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fitness/internal/auth"
	"fitness/internal/middleware"
	"fitness/internal/model"
)

// ginCtxWithAdmin 构造带管理员 claims 的请求上下文
func ginCtxWithAdmin(adminID int64, username string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Set(middleware.CtxClaims, &auth.Claims{
		UserID: adminID, Role: auth.RoleAdmin, Username: username,
	})
	return c
}

// 审计：Log 落库（含操作人/IP/详情），List 按 action 过滤
func TestAuditLog(t *testing.T) {
	svc, d := newOrderSvc(t)
	_ = svc
	ctx := context.Background()
	require.NoError(t, d.AutoMigrate(&model.AdminAuditLog{}))
	audit := NewAuditService(d)

	c := ginCtxWithAdmin(7, "boss")
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	audit.Log(c, "refund.approve", "refund", 42, "退款单审批通过，金额 100 分")
	audit.Log(c, "coupon.grant", "coupon", 1, "定向发券")

	rows, total, err := audit.List(ctx, 0, "", "", "", 1, 20)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, rows, 2)
	assert.Equal(t, "coupon.grant", rows[0].Action, "倒序：最新在前")
	assert.Equal(t, "refund.approve", rows[1].Action)
	assert.Equal(t, int64(7), rows[1].AdminID)
	assert.Equal(t, "boss", rows[1].Username)

	rows, total, err = audit.List(ctx, 7, "coupon.grant", "", "", 1, 20)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, "coupon.grant", rows[0].Action)

	// 未装配（nil）不 panic
	var nilAudit *AuditService
	nilAudit.Log(c, "x", "y", 0, "")
}

// 资料编辑：昵称必填/长度限制、头像必须站内、成功落库
func TestUpdateProfile(t *testing.T) {
	svc, d := newOrderSvc(t)
	_ = svc
	ctx := context.Background()

	_, err := UpdateUserProfile(ctx, d, 2, "  ", "")
	assert.Error(t, err, "昵称为空拒绝")
	_, err = UpdateUserProfile(ctx, d, 2, strings.Repeat("长", 70), "")
	assert.Error(t, err, "昵称超长拒绝")
	_, err = UpdateUserProfile(ctx, d, 2, "小明", "https://evil.example/a.png")
	assert.Error(t, err, "外链头像拒绝")

	u, err := UpdateUserProfile(ctx, d, 2, "爱健身的小明", "/uploads/images/202610/ab.png")
	require.NoError(t, err)
	assert.Equal(t, "爱健身的小明", u.Nickname)
	assert.Equal(t, "/uploads/images/202610/ab.png", u.AvatarURL)

	// 空头像=只改昵称
	u, err = UpdateUserProfile(ctx, d, 2, "小明", "")
	require.NoError(t, err)
	assert.NotEmpty(t, u.AvatarURL, "未传头像保持原值")
}
