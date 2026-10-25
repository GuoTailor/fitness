const app = getApp()
const { request } = require('../../utils/request')

Page({
  data: { list: [] },

  onShow() { this.load() },

  async load() {
    await app.ensureLogin()
    const rows = await request({ url: '/api/v1/qa/mine' })
    rows.forEach((q) => {
      q.timeText = String(q.createdAt || '').replace('T', ' ').slice(0, 16)
      q.statusText = q.status === 'hidden' ? '已隐藏' : '已展示'
      ;(q.answers || []).forEach((a) => {
        a.timeText = String(a.createdAt || '').replace('T', ' ').slice(0, 16)
      })
    })
    this.setData({ list: rows })
  },
})
