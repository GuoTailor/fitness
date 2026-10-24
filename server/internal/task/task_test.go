package task

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fitness/internal/config"
	"fitness/internal/db"
	"fitness/internal/model"
)

func setupTaskDB(t *testing.T) *gorm.DB {
	t.Helper()
	d, err := db.OpenSQLite()
	require.NoError(t, err)
	require.NoError(t, d.AutoMigrate(
		&model.User{}, &model.Coach{}, &model.CoursePackage{}, &model.PackageSKU{},
		&model.MemberPackage{}, &model.PackageTransaction{}, &model.Booking{},
		&model.GroupClass{}, &model.GroupEnrollment{}, &model.Verification{},
		&model.TrainingRecord{}, &model.Review{}, &model.NotifyLog{}, &model.AppConfig{},
	))
	return d
}

// 定时任务：评价邀请（核销满 N 天未评价）+ 课时/有效期预警；重复扫描不重发。
func TestRunnerScans(t *testing.T) {
	d := setupTaskDB(t)
	ctx := context.Background()

	user := model.User{OpenID: "task-user"}
	require.NoError(t, d.Create(&user).Error)
	pkg := model.CoursePackage{OwnerID: 1, Title: "任务课包", Status: 1}
	require.NoError(t, d.Create(&pkg).Error)
	// 剩 1 节（≤阈值 3）且 7 天后到期（≤阈值 14）
	mp := model.MemberPackage{
		OwnerID: 1, UserID: user.ID, PackageID: pkg.ID, PackageTitle: pkg.Title,
		TotalSessions: 5, UsedSessions: 4, Status: model.MPStatusActive,
		ExpireDate: ptrDate(time.Now().AddDate(0, 0, 7)),
	}
	require.NoError(t, d.Create(&mp).Error)
	// 3 天前已核销、未评价 → 评价邀请
	verifiedAt := time.Now().AddDate(0, 0, -3)
	v := model.Verification{
		BizType: "private", UserID: user.ID, MemberPackageID: mp.ID,
		Code: "TASKV1", Status: model.VerifyStatusVerified, VerifiedAt: &verifiedAt,
	}
	require.NoError(t, d.Create(&v).Error)

	r := NewRunner(d, nil, config.NotifyConfig{Mock: true})
	n, err := r.ReviewInvites(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	n, err = r.MemberWarnings(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "课时不足 + 快到期各一条")

	var logs []model.NotifyLog
	d.Order("id").Find(&logs)
	require.Len(t, logs, 3)
	scenes := map[string]bool{}
	for _, l := range logs {
		scenes[l.Scene] = true
	}
	assert.True(t, scenes[model.NotifySceneReviewInvite])
	assert.True(t, scenes[model.NotifySceneSessionsWarning])
	assert.True(t, scenes[model.NotifySceneExpireWarning])

	// 重复扫描：notify_log 唯一键兜底，不再新增
	_, err = r.ReviewInvites(ctx)
	require.NoError(t, err)
	_, err = r.MemberWarnings(ctx)
	require.NoError(t, err)
	var cnt int64
	d.Model(&model.NotifyLog{}).Count(&cnt)
	assert.EqualValues(t, 3, cnt)

	// 已评价后即使没发过邀请也不再推（LEFT JOIN review 过滤）
	// ——本例邀请已发过，构造新核销验证：已评价 → 不推
	v2 := model.Verification{
		BizType: "private", UserID: user.ID, MemberPackageID: mp.ID,
		Code: "TASKV2", Status: model.VerifyStatusVerified, VerifiedAt: &verifiedAt,
	}
	require.NoError(t, d.Create(&v2).Error)
	require.NoError(t, d.Create(&model.Review{
		UserID: user.ID, TargetType: model.ReviewTargetPackage, PackageID: pkg.ID,
		VerificationID: v2.ID, Rating: 5, Content: "x", Status: model.ReviewStatusApproved,
	}).Error)
	n, err = r.ReviewInvites(ctx)
	require.NoError(t, err)
	assert.Zero(t, n, "已评价的核销不再邀请")
}

func ptrDate(t time.Time) *time.Time { return &t }
