package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fitness/internal/db"
	"fitness/internal/model"
)

func setupCoachApplyDB(t *testing.T) (*coachApplyEnv, error) {
	e := &coachApplyEnv{}
	var err error
	e.db, err = db.OpenSQLite()
	if err != nil {
		return nil, err
	}
	if err := e.db.AutoMigrate(
		&model.User{}, &model.Coach{}, &model.CoachApplication{}, &model.NotifyLog{},
	); err != nil {
		return nil, err
	}
	e.svc = NewCoachApplicationService(e.db)
	for i := int64(1); i <= 3; i++ {
		if err := e.db.Create(&model.User{ID: i, OpenID: "ou-ca-" + string(rune('0'+i))}).Error; err != nil {
			return nil, err
		}
	}
	return e, nil
}

type coachApplyEnv struct {
	db  *gorm.DB
	svc *CoachApplicationService
}

func TestCoachApplySubmitRules(t *testing.T) {
	e, err := setupCoachApplyDB(t)
	require.NoError(t, err)
	ctx := context.Background()
	cert := `["/uploads/images/202610/cert1.png"]`

	// 正常提交
	rec, err := e.svc.Submit(ctx, 1, &model.CoachApplication{
		Name: "王教练", Years: 5, CertImages: cert, Specialties: "减脂", Intro: "你好",
	})
	require.NoError(t, err)
	assert.EqualValues(t, model.CoachApplyPending, rec.Status)

	// 重复 pending 拒绝
	_, err = e.svc.Submit(ctx, 1, &model.CoachApplication{Name: "王教练", Years: 5, CertImages: cert})
	assert.Contains(t, err.Error(), "已有待审核")

	// 已是在职教练拒绝
	require.NoError(t, e.db.Create(&model.Coach{UserID: 2, Name: "现役", Status: 1}).Error)
	_, err = e.svc.Submit(ctx, 2, &model.CoachApplication{Name: "现役教练", Years: 3, CertImages: cert})
	assert.Contains(t, err.Error(), "已是教练")

	// 资质图必须站内上传
	_, err = e.svc.Submit(ctx, 3, &model.CoachApplication{Name: "李教练", Years: 2, CertImages: `["https://evil.com/x.png"]`})
	assert.Contains(t, err.Error(), "站内上传")

	// 无资质图拒绝
	_, err = e.svc.Submit(ctx, 3, &model.CoachApplication{Name: "李教练", Years: 2, CertImages: ""})
	assert.Contains(t, err.Error(), "至少 1 张")
}

func TestCoachApplyReviewApprove(t *testing.T) {
	e, err := setupCoachApplyDB(t)
	require.NoError(t, err)
	ctx := context.Background()
	cert := `["/uploads/images/202610/cert1.png","/uploads/images/202610/cert2.png"]`
	rec, err := e.svc.Submit(ctx, 1, &model.CoachApplication{
		Name: "赵教练", Years: 8, CertImages: cert, Intro: "8 年经验",
	})
	require.NoError(t, err)

	approved, err := e.svc.Review(ctx, 9, rec.ID, true, "")
	require.NoError(t, err)
	assert.EqualValues(t, model.CoachApplyApproved, approved.Status)

	// 自动建档并绑定账号：登录即 isCoach
	var coach model.Coach
	require.NoError(t, e.db.Where("user_id = ?", 1).First(&coach).Error)
	assert.Equal(t, "赵教练", coach.Name)
	assert.EqualValues(t, 1, coach.Status)
	assert.EqualValues(t, 1, coach.CertVerified)

	// 审核幂等：再审返回「已被处理」
	_, err = e.svc.Review(ctx, 9, rec.ID, true, "")
	assert.Contains(t, err.Error(), "已被处理")

	// Mine 显示最新状态
	out, err := e.svc.Mine(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, true, out["exists"])
}

func TestCoachApplyReviewReject(t *testing.T) {
	e, err := setupCoachApplyDB(t)
	require.NoError(t, err)
	ctx := context.Background()
	cert := `["/uploads/images/202610/cert1.png"]`
	rec, err := e.svc.Submit(ctx, 1, &model.CoachApplication{Name: "钱教练", Years: 2, CertImages: cert})
	require.NoError(t, err)

	_, err = e.svc.Review(ctx, 9, rec.ID, false, "资质材料不清晰")
	require.NoError(t, err)
	var got model.CoachApplication
	require.NoError(t, e.db.First(&got, rec.ID).Error)
	assert.EqualValues(t, model.CoachApplyRejected, got.Status)
	assert.Equal(t, "资质材料不清晰", got.RejectReason)

	// 无教练档案生成
	var n int64
	require.NoError(t, e.db.Model(&model.Coach{}).Where("user_id = ?", 1).Count(&n).Error)
	assert.Equal(t, int64(0), n)

	// 驳回后可重新提交
	rec2, err := e.svc.Submit(ctx, 1, &model.CoachApplication{Name: "钱教练", Years: 2, CertImages: cert})
	require.NoError(t, err)
	assert.EqualValues(t, model.CoachApplyPending, rec2.Status)
}
