const app = getApp()
const { request } = require('../../utils/request')

const STATUS_TEXT = { usable: '可使用', used: '已使用', expired: '已过期' }

Page({
  data: { tab: 'usable', items: [], loading: true },

  onShow() { this.load() },

  onTab(e) {
    this.setData({ tab: e.currentTarget.dataset.tab })
    this.load()
  },

  async load() {
    await app.ensureLogin()
    this.setData({ loading: true })
    try {
      const data = await request({ url: `/api/v1/coupons/mine?status=${this.data.tab}` })
      const items = (data.items || []).map((x) => ({
        ...x,
        statusText: STATUS_TEXT[x.status] || x.status,
        expireText: x.expireAt ? String(x.expireAt).replace('T', ' ').slice(0, 10) + ' 前有效' : '',
      }))
      this.setData({ items })
    } finally {
      this.setData({ loading: false })
    }
  },

  onGoCenter() {
    wx.navigateTo({ url: '/pages/coupon/center' })
  },
})
