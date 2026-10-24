const app = getApp()
const { request } = require('../../utils/request')

Page({
  data: {
    id: 0,
    detail: null,
    selectedSku: -1,
    buying: false,
    reviewTab: 'reviews',
  },

  onLoad(query) {
    this.setData({ id: query.id })
    this.load()
  },

  async load() {
    const data = await request({ url: `/api/v1/packages/${this.data.id}` })
    // 价格分转元展示
    data.skus.forEach((s) => { s.priceYuan = (s.price / 100).toFixed(0) })
    this.setData({ detail: data, selectedSku: data.skus.length === 1 ? 0 : -1 })
  },

  onSku(e) {
    this.setData({ selectedSku: Number(e.currentTarget.dataset.index) })
  },
  onTab(e) {
    this.setData({ reviewTab: e.currentTarget.dataset.tab })
  },
  onCoach(e) {
    wx.navigateTo({ url: '/pages/coach/detail?id=' + e.currentTarget.dataset.id })
  },

  async onBuy() {
    const app2 = app
    await app2.ensureLogin()
    const { detail, selectedSku } = this.data
    if (selectedSku < 0) return wx.showToast({ title: '请选择规格', icon: 'none' })
    const sku = detail.skus[selectedSku]
    this.setData({ buying: true })
    try {
      const order = await request({
        url: '/api/v1/orders', method: 'POST', data: { skuId: sku.id, couponId: 0 },
      })
      // 支付：mock 通道直接成功；真实通道返回 payParams 拉起 wx.requestPayment
      const payRes = await request({
        url: `/api/v1/orders/${order.id}/prepay`, method: 'POST', data: {},
      })
      if (payRes.mockPaid) {
        wx.showModal({
          title: '支付成功',
          content: `已购买「${order.title}」，课时已入账`,
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
