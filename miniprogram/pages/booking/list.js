const app = getApp()
const { request } = require('../../utils/request')

const STATUS = {
  pending: ['待确认', '#e6a23c'], confirmed: ['已确认', '#00b578'],
  completed: ['已完成', '#999'], cancelled: ['已取消', '#bbb'], expired: ['已过期', '#bbb'],
}
const WEEK = ['日', '一', '二', '三', '四', '五', '六']

Page({
  data: { list: [] },

  onShow() { this.load() },

  async load() {
    const app2 = app
    await app2.ensureLogin()
    const list = await request({ url: '/api/v1/bookings' })
    list.forEach((b) => {
      const [text, color] = STATUS[b.status] || [b.status, '#999']
      b.statusText = text
      b.statusColor = color
      b.timeText = `${b.bookingDate} 周${WEEK[new Date(b.bookingDate).getDay()]} ${fmt(b.startMinute)}-${fmt(b.endMinute)}`
    })
    this.setData({ list })
  },

  async onCancel(e) {
    const id = e.currentTarget.dataset.id
    const { confirm } = await new Promise((r) => wx.showModal({ title: '取消预约', content: '开课前 4 小时内取消将扣 1 课时，确认取消？', success: r }))
    if (!confirm) return
    await request({ url: `/api/v1/bookings/${id}/cancel`, method: 'POST' })
    wx.showToast({ title: '已取消', icon: 'success' })
    this.load()
  },

  onDetail(e) {
    wx.navigateTo({ url: '/pages/booking/detail?id=' + e.currentTarget.dataset.id })
  },
})

function fmt(m) {
  return `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`
}
