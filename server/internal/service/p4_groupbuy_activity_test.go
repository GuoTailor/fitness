package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fitness/internal/model"
)

// ---- 测试辅助 ----

func mustCreateActivity(t *testing.T, e *gbEnv, price int64, capacity int, start, end time.Time, maxGroups, expireHours, status int) *model.GroupBuyActivity {
	t.Helper()
	act := model.GroupBuyActivity{
		Title: "测试拼团活动", SkuID: e.sku.ID, PackageID: e.pkg.ID,
		GroupPrice: price, GroupCapacity: capacity,
		StartAt: start, EndAt: end, MaxGroups: maxGroups, ExpireHours: expireHours, Status: int8(status),
	}
	require.NoError(t, e.db.Create(&act).Error)
	if status == 0 { // default:1 标签会吞掉 Create 的零值 status，下架态需回填
		require.NoError(t, e.db.Model(&model.GroupBuyActivity{}).Where("id = ?", act.ID).
			Update("status", 0).Error)
	}
	return &act
}

func mustCreateSkuActivity(t *testing.T, e *gbEnv, sku *model.PackageSKU, price int64, capacity int, start, end time.Time, status int) *model.GroupBuyActivity {
	t.Helper()
	act := model.GroupBuyActivity{
		Title: "规格专属活动", SkuID: sku.ID, PackageID: sku.PackageID,
		GroupPrice: price, GroupCapacity: capacity, StartAt: start, EndAt: end, Status: int8(status),
	}
	require.NoError(t, e.db.Create(&act).Error)
	if status == 0 {
		require.NoError(t, e.db.Model(&model.GroupBuyActivity{}).Where("id = ?", act.ID).
			Update("status", 0).Error)
	}
	return &act
}

func mustCreateSku(t *testing.T, e *gbEnv, name string, price, groupPrice int64, groupCapacity int) *model.PackageSKU {
	t.Helper()
	sku := model.PackageSKU{PackageID: e.pkg.ID, Name: name, Sessions: 5, ValidDays: 30,
		Price: price, GroupPrice: groupPrice, GroupCapacity: groupCapacity, Status: 1}
	require.NoError(t, e.db.Create(&sku).Error)
	return &sku
}

func openedCountOf(t *testing.T, e *gbEnv, actID int64) int {
	t.Helper()
	var act model.GroupBuyActivity
	require.NoError(t, e.db.First(&act, actID).Error)
	return act.OpenedCount
}

// skuJSONFrom 把 PackageDetail 的 skus 转回可断言的线格式（skuView 是函数内局部类型，走 JSON 往返）
type skuWire struct {
	ID                 int64  `json:"id"`
	GroupPrice         int64  `json:"groupPrice"`
	GroupCapacity      int    `json:"groupCapacity"`
	GroupActivityId    int64  `json:"groupActivityId"`
	GroupActivityTitle string `json:"groupActivityTitle"`
}

func skuJSONFrom(t *testing.T, detail map[string]interface{}) []skuWire {
	t.Helper()
	b, err := json.Marshal(detail["skus"])
	require.NoError(t, err)
	var skus []skuWire
	require.NoError(t, json.Unmarshal(b, &skus))
	require.NotEmpty(t, skus)
	return skus
}

// ---- 用例 ----

func TestGroupBuyActivityOpen(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()
	now := time.Now()
	act := mustCreateActivity(t, e, 49900, 2, now.Add(-time.Hour), now.Add(24*time.Hour), 0, 6, 1)

	o, err := e.gb.Open(ctx, 1, e.sku.ID)
	require.NoError(t, err)
	// 活动价/人数生效，团关联活动
	assert.Equal(t, int64(49900), o.Amount)
	var gb model.GroupBuy
	require.NoError(t, e.db.First(&gb, o.GroupBuyID).Error)
	assert.Equal(t, act.ID, gb.ActivityID)
	assert.Equal(t, 2, gb.Capacity)
	// 每团有效期取活动的 6 小时（而非全局 48）
	assert.True(t, gb.ExpireAt.After(now.Add(6*time.Hour-time.Second)), "expireAt=%v", gb.ExpireAt)
	assert.True(t, gb.ExpireAt.Before(now.Add(7*time.Hour)), "expireAt=%v", gb.ExpireAt)
	// 名额计数器 +1
	assert.Equal(t, 1, openedCountOf(t, e, act.ID))
}

func TestGroupBuyActivityWindowFallback(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()
	now := time.Now()

	// 未开始 → 回落 SKU 常开配置
	mustCreateActivity(t, e, 49900, 2, now.Add(2*time.Hour), now.Add(24*time.Hour), 0, 0, 1)
	o, err := e.gb.Open(ctx, 1, e.sku.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(66000), o.Amount)

	// 已结束 → 回落
	mustCreateActivity(t, e, 39900, 2, now.Add(-24*time.Hour), now.Add(-time.Hour), 0, 0, 1)
	o2, err := e.gb.Open(ctx, 2, e.sku.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(66000), o2.Amount)

	// 下架 → 回落
	mustCreateActivity(t, e, 39900, 2, now.Add(-time.Hour), now.Add(24*time.Hour), 0, 0, 0)
	o3, err := e.gb.Open(ctx, 3, e.sku.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(66000), o3.Amount)

	// SKU 未配拼团且无活动 → 未开启拼团
	nsku := mustCreateSku(t, e, "无拼团规格", 50000, 0, 0)
	_, err = e.gb.Open(ctx, 4, nsku.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未开启拼团")

	// SKU 未配拼团但活动生效 → 活动独立开启拼团
	now2 := time.Now()
	mustCreateSkuActivity(t, e, nsku, 39900, 2, now2.Add(-time.Hour), now2.Add(24*time.Hour), 1)
	o4, err := e.gb.Open(ctx, 4, nsku.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(39900), o4.Amount)
}

func TestGroupBuyActivityQuota(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()
	now := time.Now()
	// max_groups=1：第一团占坑后第二团被拒
	act := mustCreateActivity(t, e, 49900, 2, now.Add(-time.Hour), now.Add(24*time.Hour), 1, 0, 1)

	_, err := e.gb.Open(ctx, 1, e.sku.ID)
	require.NoError(t, err)
	_, err = e.gb.Open(ctx, 2, e.sku.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "名额已满")

	// 第一团到期失败退款也不返还名额（开团即占坑）
	o1, err := e.gb.Open(ctx, 3, e.sku.ID)
	require.Error(t, err) // 上一步已占满，这里不会建出新单
	_ = o1
	assert.Equal(t, 1, openedCountOf(t, e, act.ID))
}

func TestGroupBuyActivityQuotaWithRefund(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()
	now := time.Now()
	act := mustCreateActivity(t, e, 49900, 2, now.Add(-time.Hour), now.Add(24*time.Hour), 1, 0, 1)

	o1, err := e.gb.Open(ctx, 1, e.sku.ID)
	require.NoError(t, err)
	_, err = e.orders.Prepay(ctx, o1.ID, 1)
	require.NoError(t, err)
	// 到期未成团 → 自动退款，名额不返还
	require.NoError(t, e.db.Model(&model.GroupBuy{}).Where("id = ?", o1.GroupBuyID).
		Update("expire_at", time.Now().Add(-time.Hour)).Error)
	expireGroupBuysForTest(t, e)
	_, err = e.gb.Open(ctx, 2, e.sku.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "名额已满")
	assert.Equal(t, 1, openedCountOf(t, e, act.ID))
}

func TestGroupBuyActivityPrecedence(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()
	now := time.Now()
	// 活动优先于 SKU 常开配置（价可高于 SKU 拼团价，方向不限）
	mustCreateActivity(t, e, 88000, 3, now.Add(-time.Hour), now.Add(24*time.Hour), 0, 0, 1)
	o, err := e.gb.Open(ctx, 1, e.sku.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(88000), o.Amount)
}

func TestActivitySaveValidation(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()
	now := time.Now()
	valid := ActivityInput{
		Title: "周年庆拼团", SkuID: e.sku.ID, GroupPrice: 49900, GroupCapacity: 2,
		StartAt: now.Add(-time.Hour), EndAt: now.Add(24 * time.Hour), Status: 1,
	}

	// 基础校验
	bad := valid
	bad.Title = " "
	_, err := e.gb.ActivitySave(ctx, &bad)
	assert.Contains(t, err.Error(), "标题")

	bad = valid
	bad.GroupCapacity = 6
	_, err = e.gb.ActivitySave(ctx, &bad)
	assert.Contains(t, err.Error(), "2~5")

	bad = valid
	bad.GroupPrice = e.sku.Price
	_, err = e.gb.ActivitySave(ctx, &bad)
	assert.Contains(t, err.Error(), "低于规格售价")

	bad = valid
	bad.EndAt = bad.StartAt
	_, err = e.gb.ActivitySave(ctx, &bad)
	assert.Contains(t, err.Error(), "结束时间")

	// 时间不重叠：正常创建
	act, err := e.gb.ActivitySave(ctx, &valid)
	require.NoError(t, err)

	// 同规格时间重叠拒绝（无论对方是否下架）
	overlap := valid
	overlap.Title = "重叠活动"
	overlap.StartAt = now
	overlap.EndAt = now.Add(5 * time.Hour)
	_, err = e.gb.ActivitySave(ctx, &overlap)
	assert.Contains(t, err.Error(), "已有拼团活动")

	// 不重叠的时间段可再建
	later := valid
	later.Title = "下一场"
	later.StartAt = now.Add(25 * time.Hour)
	later.EndAt = now.Add(48 * time.Hour)
	_, err = e.gb.ActivitySave(ctx, &later)
	require.NoError(t, err)

	// 已开团禁改规格；同规格继续编辑允许
	nsku := mustCreateSku(t, e, "另一规格", 80000, 0, 0)
	_, err = e.gb.Open(ctx, 1, e.sku.ID)
	require.NoError(t, err)
	edit := valid
	edit.ID = act.ID
	edit.SkuID = nsku.ID
	_, err = e.gb.ActivitySave(ctx, &edit)
	assert.Contains(t, err.Error(), "不允许更换规格")

	edit.SkuID = e.sku.ID
	edit.GroupPrice = 45900
	saved, err := e.gb.ActivitySave(ctx, &edit)
	require.NoError(t, err)
	assert.Equal(t, int64(45900), saved.GroupPrice)
}

func TestActivityDelete(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()
	now := time.Now()

	// 未开团可删
	act := mustCreateActivity(t, e, 49900, 2, now.Add(-time.Hour), now.Add(24*time.Hour), 0, 0, 1)
	require.NoError(t, e.gb.ActivityDelete(ctx, act.ID))
	var n int64
	require.NoError(t, e.db.Model(&model.GroupBuyActivity{}).Where("id = ?", act.ID).Count(&n).Error)
	assert.Zero(t, n)

	// 已开团禁删
	act2 := mustCreateActivity(t, e, 49900, 2, now.Add(-time.Hour), now.Add(24*time.Hour), 0, 0, 1)
	_, err := e.gb.Open(ctx, 1, e.sku.ID)
	require.NoError(t, err)
	err = e.gb.ActivityDelete(ctx, act2.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不能删除")
}

func TestPackageDetailActivityOverride(t *testing.T) {
	e := gbFixture(t)
	require.NoError(t, e.db.AutoMigrate(&model.Review{}, &model.Coach{}, &model.CoachPackage{}))
	ctx := context.Background()
	shop := NewShopService(e.db, 1)

	// 无活动：SKU 常开配置原样展示
	detail, err := shop.PackageDetail(ctx, e.pkg.ID, 0)
	require.NoError(t, err)
	skus := skuJSONFrom(t, detail)
	assert.Equal(t, int64(66000), skus[0].GroupPrice)
	assert.Equal(t, 3, skus[0].GroupCapacity)
	assert.Zero(t, skus[0].GroupActivityId)

	// 有活动：覆盖拼团价/人数 + 活动信息
	now := time.Now()
	mustCreateActivity(t, e, 49900, 2, now.Add(-time.Hour), now.Add(24*time.Hour), 0, 0, 1)
	detail, err = shop.PackageDetail(ctx, e.pkg.ID, 0)
	require.NoError(t, err)
	skus = skuJSONFrom(t, detail)
	assert.Equal(t, int64(49900), skus[0].GroupPrice)
	assert.Equal(t, 2, skus[0].GroupCapacity)
	assert.NotZero(t, skus[0].GroupActivityId)
	assert.NotEmpty(t, skus[0].GroupActivityTitle)
}
