const app = getApp()
const { request } = require('../../utils/request')

Page({
  data: {
    id: 0,
    detail: null,
    selectedSku: -1,
    coupons: [],       // 当前 SKU 可用券（后端已按门槛/范围过滤）
    couponIndex: -1,   // -1=不使用优惠券
    payYuan: '',       // 预估合计（新人价与券取低，最终以下单服务端为准）
    buying: false,
    reviewTab: 'reviews',
  },

  onLoad(query) {
    this.setData({ id: query.id })
    this.load()
  },

  async load() {
    const data = await request({ url: `/api/v1/packages/${this.data.id}` })
    // 价格分转元展示；新人价仅对可享用户展示（服务端判定）
    data.skus.forEach((s) => {
      s.priceYuan = (s.price / 100).toFixed(0)
      if (s.newcomerPrice && s.newcomerEligible) {
        s.newcomerYuan = (s.newcomerPrice / 100).toFixed(0)
      }
    })
    this.setData({ detail: data, selectedSku: data.skus.length === 1 ? 0 : -1 })
    if (data.skus.length === 1) this.loadCoupons(data.skus[0].id)
  },

  onSku(e) {
    this.setData({ selectedSku: Number(e.currentTarget.dataset.index) })
    const sku = this.data.detail.skus[this.data.selectedSku]
    this.loadCoupons(sku.id)
  },
  onTab(e) {
    this.setData({ reviewTab: e.currentTarget.dataset.tab })
  },
  onCoach(e) {
    wx.navigateTo({ url: '/pages/coach/detail?id=' + e.currentTarget.dataset.id })
  },

  async loadCoupons(skuId) {
    try {
      const data = await request({ url: `/api/v1/coupons/usable?skuId=${skuId}` })
      const items = (data.items || []).map((c) => ({ ...c, benefitYuan: (c.benefit / 100).toFixed(2) }))
      this.setData({ coupons: items, couponIndex: items.length ? 0 : -1 })
    } catch (e) {
      this.setData({ coupons: [], couponIndex: -1 })
    }
    this.computePay()
  },

  async onPickCoupon() {
    const { coupons } = this.data
    if (!coupons.length) {
      return wx.showToast({ title: '该课包暂无可用券，可去领券中心看看', icon: 'none' })
    }
    const items = ['不使用优惠券'].concat(
      coupons.map((c) => `${c.name} ${c.label}（省¥${c.benefitYuan}）`))
    wx.showActionSheet({
      itemList: items,
      success: (r) => {
        this.setData({ couponIndex: r.tapIndex - 1 })
        this.computePay()
      },
    })
  },
  onGoCouponCenter() {
    wx.navigateTo({ url: '/pages/coupon/center' })
  },

  computePay() {
    const { detail, selectedSku, couponIndex, coupons } = this.data
    if (!detail || selectedSku < 0) {
      this.setData({ payYuan: '' })
      return
    }
    const sku = detail.skus[selectedSku]
    let pay = sku.price
    if (couponIndex >= 0 && coupons[couponIndex]) pay = coupons[couponIndex].finalPay
    if (sku.newcomerEligible && sku.newcomerPrice && sku.newcomerPrice < pay) pay = sku.newcomerPrice
    this.setData({ payYuan: (pay / 100).toFixed(2) })
  },

  async onBuy() {
    const app2 = app
    await app2.ensureLogin()
    const { detail, selectedSku, coupons, couponIndex } = this.data
    if (selectedSku < 0) return wx.showToast({ title: '请选择规格', icon: 'none' })
    const sku = detail.skus[selectedSku]
    const couponId = couponIndex >= 0 && coupons[couponIndex] ? coupons[couponIndex].couponId : 0
    this.setData({ buying: true })
    try {
      const order = await request({
        url: '/api/v1/orders', method: 'POST', data: { skuId: sku.id, couponId },
      })
      // 支付：mock 通道直接成功；真实通道返回 payParams 拉起 wx.requestPayment
      const payRes = await request({
        url: `/api/v1/orders/${order.id}/prepay`, method: 'POST', data: {},
      })
      if (payRes.mockPaid) {
        wx.showModal({
          title: '支付成功',
          content: `已购买「${order.title}」，实付 ¥${(order.amount / 100).toFixed(2)}`,
          confirmText: '去预约',
          cancelText: '看课包',
          success: (r) => {
            if (r.confirm) wx.navigateTo({ url: '/pages/training/index' })
          },
        })
      } else if (payRes.payParams) {
        const p = payRes.payParams
        await new Promise((resolve, reject) => {
          wx.requestPayment({
            timeStamp: p.timeStamp, nonceStr: p.nonceStr, package: p.package,
            signType: p.signType, paySign: p.paySign,
            success: resolve, fail: reject,
          })
        })
        wx.showModal({
          title: '支付成功',
          content: `已购买「${order.title}」，课时将在支付回调后入账`,
          showCancel: false,
        })
      } else {
        wx.showToast({ title: '支付通道未就绪', icon: 'none' })
      }
    } catch (e) {
      // request 封装已 toast
    } finally {
      this.setData({ buying: false })
    }
  },
})
