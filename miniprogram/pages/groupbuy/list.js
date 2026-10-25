// 拼团列表页：某课包规格的进行中团 + 开新团
const app = getApp()
const { request } = require('../../utils/request')
const { payOrder } = require('../../utils/pay')

Page({
  data: {
    skuId: 0, title: '', sku: null,
    groups: [], opening: false,
  },

  onLoad(query) {
    this.setData({ skuId: parseInt(query.skuId || 0, 10), pkgId: parseInt(query.pkgId || 0, 10) })
    this.load()
  },

  async load() {
    await app.ensureLogin()
    if (this.data.pkgId) {
      const detail = await request({ url: `/api/v1/packages/${this.data.pkgId}` }).catch(() => null)
      if (detail) {
        const sku = (detail.skus || []).find((s) => s.id === this.data.skuId)
        if (sku) this.setData({ title: detail.title, sku })
      }
    }
    const res = await request({ url: `/api/v1/group-buys?skuId=${this.data.skuId}` })
    const groups = res.list || []
    groups.forEach((g) => {
      g.priceYuan = (g.price / 100).toFixed(0)
      g.endText = String(g.expireAt).replace('T', ' ').slice(5, 16)
    })
    this.setData({ groups })
  },

  async onOpen() {
    this.setData({ opening: true })
    try {
      const order = await request({
        url: '/api/v1/group-buys', method: 'POST', data: { skuId: this.data.skuId },
      })
      const payRes = await payOrder(order.id)
      if (payRes.mockPaid || payRes.paid) {
        wx.showToast({ title: '开团成功，快邀请好友', icon: 'none' })
        wx.redirectTo({ url: `/pages/groupbuy/detail?gbid=${order.groupBuyId}` })
      }
    } catch (e) { /* toast 已处理 */ } finally {
      this.setData({ opening: false })
    }
  },

  goDetail(e) {
    wx.navigateTo({ url: `/pages/groupbuy/detail?gbid=${e.currentTarget.dataset.id}` })
  },
})
