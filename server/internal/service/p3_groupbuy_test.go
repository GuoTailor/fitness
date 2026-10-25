package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fitness/internal/db"
	"fitness/internal/model"
	"fitness/internal/wechat"
)

type gbEnv struct {
	db     *gorm.DB
	cfg    *ConfigService
	gb     *GroupBuyService
	orders *OrderService
	pkg    *model.CoursePackage
	sku    *model.PackageSKU
}

func gbFixture(t *testing.T) *gbEnv {
	t.Helper()
	d, err := db.OpenSQLite()
	require.NoError(t, err)
	require.NoError(t, d.AutoMigrate(
		&model.User{}, &model.CoursePackage{}, &model.PackageSKU{},
		&model.Order{}, &model.Payment{}, &model.Refund{},
		&model.MemberPackage{}, &model.PackageTransaction{}, &model.AppConfig{},
		&model.GroupBuy{}, &model.GroupBuyMember{}, &model.NotifyLog{},
		&model.CouponTemplate{}, &model.UserCoupon{},
	))
	cfgSvc := NewConfigService(d)
	gbSvc := NewGroupBuyService(d, cfgSvc)
	orders := NewOrderService(d, cfgSvc, wechat.NewMockPay()).WithGroupBuy(gbSvc)
	for i := int64(1); i <= 5; i++ {
		require.NoError(t, d.Create(&model.User{ID: i, OpenID: "ou-gb-" + string(rune('0'+i))}).Error)
	}
	pkg := model.CoursePackage{OwnerID: 1, Title: "拼团测试课包", Category: model.PkgCategoryGroup, Status: 1}
	require.NoError(t, d.Create(&pkg).Error)
	sku := model.PackageSKU{PackageID: pkg.ID, Name: "10次卡", Sessions: 10, ValidDays: 90, Price: 100000, GroupPrice: 66000, GroupCapacity: 3, Status: 1}
	require.NoError(t, d.Create(&sku).Error)
	return &gbEnv{db: d, cfg: cfgSvc, gb: gbSvc, orders: orders, pkg: &pkg, sku: &sku}
}

func TestGroupBuyOpenJoinSucceed(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()

	// 开团 → 待支付拼团单
	o1, err := e.gb.Open(ctx, 1, e.sku.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(66000), o1.Amount)
	assert.NotZero(t, o1.GroupBuyID)
	// 重复开团：同一用户已占成员位，再开第二团不受限（每人可开多团），但同团重复参团被拒
	o1b, err := e.gb.Open(ctx, 1, e.sku.ID)
	require.NoError(t, err, "开多个团允许")

	// 未支付订单不计数；第 2 人参团支付
	o2, err := e.gb.Join(ctx, 2, o1.GroupBuyID)
	require.NoError(t, err)
	// 同人重复参团拒绝
	_, err = e.gb.Join(ctx, 2, o1.GroupBuyID)
	assert.Contains(t, err.Error(), "已参与该团")
	// 支付 2 单
	_, err = e.orders.Prepay(ctx, o1.ID, 1)
	require.NoError(t, err)
	_, err = e.orders.Prepay(ctx, o2.ID, 2)
	require.NoError(t, err)
	// 2/3 未达 capacity：仍报名中
	var gb model.GroupBuy
	require.NoError(t, e.db.First(&gb, o1.GroupBuyID).Error)
	assert.EqualValues(t, model.GroupBuyEnrolling, gb.Status)

	// 第 3 人支付 → 自动成团
	o3, err := e.gb.Join(ctx, 3, o1.GroupBuyID)
	require.NoError(t, err)
	_, err = e.orders.Prepay(ctx, o3.ID, 3)
	require.NoError(t, err)
	require.NoError(t, e.db.First(&gb, o1.GroupBuyID).Error)
	assert.EqualValues(t, model.GroupBuySuccess, gb.Status)
	assert.NotNil(t, gb.SuccessAt)

	// 成团后参团拒绝
	_, err = e.gb.Join(ctx, 4, o1.GroupBuyID)
	assert.Contains(t, err.Error(), "已结束")

	// 三人的课时均已入账
	for _, uid := range []int64{1, 2, 3} {
		var mp model.MemberPackage
		require.NoError(t, e.db.Where("user_id = ? AND order_id IN (?)", uid,
			e.db.Model(&model.Order{}).Select("id").Where("user_id = ?", uid)).First(&mp).Error)
		assert.Equal(t, 10, mp.TotalSessions)
	}
	_ = o1b
}

func TestGroupBuyExpiryAutoRefund(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()
	o1, err := e.gb.Open(ctx, 1, e.sku.ID)
	require.NoError(t, err)
	_, err = e.orders.Prepay(ctx, o1.ID, 1)
	require.NoError(t, err)

	// 回填过期时间 → 到期置 failed 并自动退款（直接调任务函数，等价 Runner.CloseGroupBuys 内部逻辑）
	require.NoError(t, e.db.Model(&model.GroupBuy{}).Where("id = ?", o1.GroupBuyID).
		Update("expire_at", time.Now().Add(-time.Hour)).Error)
	expireGroupBuysForTest(t, e)
	var gb model.GroupBuy
	require.NoError(t, e.db.First(&gb, o1.GroupBuyID).Error)
	assert.EqualValues(t, model.GroupBuyFailed, gb.Status)

	// 订单已自动退款
	var order model.Order
	require.NoError(t, e.db.First(&order, o1.ID).Error)
	assert.Equal(t, model.OrderStatusRefunded, order.Status)
	assert.Equal(t, int64(66000), order.RefundAmount)

	// 已入账课时被回收（ApproveRefund 链路）
	var mp model.MemberPackage
	require.NoError(t, e.db.Where("order_id = ?", o1.ID).First(&mp).Error)
	assert.Equal(t, model.MPStatusRefunded, mp.Status)

	// 到期任务幂等：再跑不重复处理
	expireGroupBuysForTest(t, e)
	require.NoError(t, e.db.First(&order, o1.ID).Error)
	assert.Equal(t, model.OrderStatusRefunded, order.Status)
}

// expireGroupBuysForTest 与 Runner.CloseGroupBuys 相同的到期逻辑（单测不引入 task 包循环依赖）
func expireGroupBuysForTest(t *testing.T, e *gbEnv) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	var gbs []model.GroupBuy
	require.NoError(t, e.db.WithContext(ctx).
		Where("status = ? AND expire_at <= ?", model.GroupBuyEnrolling, now).
		Find(&gbs).Error)
	for _, gb := range gbs {
		res := e.db.WithContext(ctx).Model(&model.GroupBuy{}).
			Where("id = ? AND status = ?", gb.ID, model.GroupBuyEnrolling).
			Update("status", model.GroupBuyFailed)
		require.NoError(t, res.Error)
		if res.RowsAffected == 0 {
			continue
		}
		var orders []model.Order
		require.NoError(t, e.db.WithContext(ctx).
			Where("group_buy_id = ? AND status = ?", gb.ID, model.OrderStatusPaid).
			Find(&orders).Error)
		for _, o := range orders {
			refund, err := e.orders.CreateAutoRefund(ctx, o.ID, o.UserID, "拼团未成团，自动退款")
			require.NoError(t, err)
			_, err = e.orders.ApproveRefund(ctx, refund.ID, 0)
			require.NoError(t, err)
		}
	}
}

func TestGroupBuyValidation(t *testing.T) {
	e := gbFixture(t)
	ctx := context.Background()

	// 未开启拼团的 SKU
	nsku := model.PackageSKU{PackageID: e.pkg.ID, Name: "无拼团", Sessions: 5, ValidDays: 30, Price: 50000, Status: 1}
	require.NoError(t, e.db.Create(&nsku).Error)
	_, err := e.gb.Open(ctx, 1, nsku.ID)
	assert.Contains(t, err.Error(), "未开启拼团")

	// 满员后参团拒绝：capacity=3 但只允许成员 3 人 → 手动造 3 个已支付成员
	o, err := e.gb.Open(ctx, 1, e.sku.ID)
	require.NoError(t, err)
	gbID := o.GroupBuyID
	for uid := int64(2); uid <= 3; uid++ {
		_, err := e.gb.Join(ctx, uid, gbID)
		require.NoError(t, err)
	}
	_, err = e.gb.Join(ctx, 4, gbID)
	assert.Contains(t, err.Error(), "满员")
}
