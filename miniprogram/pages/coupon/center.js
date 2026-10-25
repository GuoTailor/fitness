const app = getApp()
const { request } = require('../../utils/request')

Page({
  data: { items: [], loading: true },

  onShow() { this.load() },

  async load() {
    await app.ensureLogin()
    this.setData({ loading: true })
    try {
      const data = await request({ url: '/api/v1/coupons/center' })
      const items = (data.items || []).map((x) => ({
        ...x,
        validText: `领取后 ${x.validDays} 天内有效`,
        scopeText: x.packageId ? '限指定课包' : '全场通用',
      }))
      this.setData({ items })
    } finally {
      this.setData({ loading: false })
    }
  },

  async onClaim(e) {
    const item = this.data.items[Number(e.currentTarget.dataset.index)]
    if (!item || !item.claimable) return
    await request({ url: `/api/v1/coupons/${item.templateId}/claim`, method: 'POST', data: {} })
    wx.showToast({ title: '领取成功', icon: 'success' })
    this.load()
  },
})
