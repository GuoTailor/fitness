package service

// P1 二期 · 批次④ 单测：课程问答区（权限矩阵/排序/隐藏/置顶）

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fitness/internal/model"
)

func setupQa(t *testing.T) (*QaService, *fixture) {
	t.Helper()
	f := newFixture(t, 10, 9*60, 21*60)
	require.NoError(t, f.db.AutoMigrate(&model.QaQuestion{}, &model.QaAnswer{}, &model.Order{}))
	return NewQaService(f.db), f
}

// 问答权限矩阵与排序：教练回复带标识优先、已购学员可回、陌生人不可回、匿名展示、后台隐藏
func TestQaFlow(t *testing.T) {
	qa, f := setupQa(t)
	ctx := context.Background()
	pkgID := pkgIDOf(f)

	// 教练绑定登录账号（回答走 user_id）
	coachUser := model.User{OpenID: "qa-coach"}
	require.NoError(t, f.db.Create(&coachUser).Error)
	require.NoError(t, f.db.Model(&model.Coach{}).Where("id = ?", f.coachID).Update("user_id", coachUser.ID).Error)

	// 提问：内容过短拒绝 → 正常提问（匿名）
	_, err := qa.Ask(ctx, f.userID, pkgID, "短", false)
	assert.Error(t, err)
	q, err := qa.Ask(ctx, f.userID, pkgID, "零基础可以上这个课包吗，需要自带什么装备？", true)
	require.NoError(t, err)
	assert.Equal(t, model.QaStatusVisible, q.Status, "先发后审：默认可见")

	// 陌生人（未购、非教练）不可回复
	stranger := model.User{OpenID: "qa-stranger"}
	require.NoError(t, f.db.Create(&stranger).Error)
	_, err = qa.Answer(ctx, stranger.ID, q.ID, "我不知道")
	assert.ErrorContains(t, err, "教练或已购")

	// 教练回复 → isCoach=1
	aCoach, err := qa.Answer(ctx, coachUser.ID, q.ID, "零基础完全没问题，装备馆里都有，穿运动鞋来即可")
	require.NoError(t, err)
	assert.Equal(t, int8(1), aCoach.IsCoach)

	// 已购学员可回复
	//（fixture 的 mp 不算订单；造一条 paid 订单）
	require.NoError(t, f.db.Create(&model.Order{
		OrderNo: "QAORD1", UserID: stranger.ID, PackageID: pkgID, Amount: 100,
		Status: model.OrderStatusPaid,
	}).Error)
	aBuyer, err := qa.Answer(ctx, stranger.ID, q.ID, "我也是零基础报名的，教练很耐心")
	require.NoError(t, err)
	assert.Equal(t, int8(0), aBuyer.IsCoach)

	// 公开列表：匿名提问显示「匿名学员」；教练回复排在学员前
	list, total, err := qa.ListOfPackage(ctx, pkgID, 1, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, list, 1)
	assert.Equal(t, "匿名学员", list[0]["asker"])
	answers := list[0]["answers"].([]map[string]interface{})
	require.Len(t, answers, 2)
	assert.EqualValues(t, 1, answers[0]["isCoach"], "教练回复优先展示")

	// 我的提问（真实昵称对自己可见）
	mine, err := qa.MyQuestions(ctx, f.userID)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.NotEqual(t, "匿名学员", mine[0]["asker"], "本人可见真实昵称")

	// 后台隐藏问题 → 公开列表不再出现
	require.NoError(t, qa.SetQuestionStatus(ctx, q.ID, model.QaStatusHidden))
	_, total, _ = qa.ListOfPackage(ctx, pkgID, 1, 10)
	assert.EqualValues(t, 0, total)
	// 隐藏后不可回复
	_, err = qa.Answer(ctx, coachUser.ID, q.ID, "还能回吗")
	assert.Error(t, err)
	require.NoError(t, qa.SetQuestionStatus(ctx, q.ID, model.QaStatusVisible))

	// 后台隐藏单条回答 → 列表剔除该条
	require.NoError(t, qa.SetAnswerStatus(ctx, aBuyer.ID, model.QaStatusHidden))
	list, _, _ = qa.ListOfPackage(ctx, pkgID, 1, 10)
	answers = list[0]["answers"].([]map[string]interface{})
	assert.Len(t, answers, 1)

	// 教练置顶（自己课包）→ pinned 优先排序
	q2, err := qa.Ask(ctx, stranger.ID, pkgID, "第二问：一周上几次合适？", false)
	require.NoError(t, err)
	require.NoError(t, qa.TogglePinCoach(ctx, f.coachID, q2.ID))
	list, _, _ = qa.ListOfPackage(ctx, pkgID, 1, 10)
	assert.EqualValues(t, q2.ID, list[0]["id"], "置顶问题排第一")

	// 非本课包教练置顶被拒（用另一个教练）
	otherCoach := model.Coach{OwnerID: 1, Name: "无关教练", Status: 1}
	require.NoError(t, f.db.Create(&otherCoach).Error)
	assert.Error(t, qa.TogglePinCoach(ctx, otherCoach.ID, q.ID))
}
