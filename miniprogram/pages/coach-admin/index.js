const app = getApp()
const { request } = require('../../utils/request')

const fmtM = (m) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`
const today = () => {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

Page({
  data: { date: '', day: [], stats: null },

  onShow() {
    this.setData({ date: today() })
    this.load()
  },

  async load() {
    const [day, stats] = await Promise.all([
      request({ url: `/api/v1/coach/bookings?date=${this.data.date}` }),
      request({ url: '/api/v1/coach/stats' }),
    ])
    day.forEach((r) => { r.startLabel = fmtM(r.booking.startMinute) })
    this.setData({ day, stats })
  },

  async onConfirm(e) {
    await request({ url: `/api/v1/coach/bookings/${e.currentTarget.dataset.id}/confirm`, method: 'POST' })
    wx.showToast({ title: '已确认', icon: 'success' })
    this.load()
  },

  async onReject(e) {
    const { confirm } = await new Promise((r) =>
      wx.showModal({ title: '拒绝预约', content: '拒绝后将退还学员课时并释放时段', success: r }))
    if (!confirm) return
    await request({ url: `/api/v1/coach/bookings/${e.currentTarget.dataset.id}/reject`, method: 'POST' })
    this.load()
  },

  goVerify() { wx.navigateTo({ url: '/pages/coach-admin/verify' }) },
  goSchedule() { wx.navigateTo({ url: '/pages/coach-admin/schedule' }) },
  goGroups() { wx.navigateTo({ url: '/pages/coach-admin/groups' }) },
  goReviews() { wx.navigateTo({ url: '/pages/coach-admin/reviews' }) },
})
