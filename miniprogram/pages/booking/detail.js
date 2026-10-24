const app = getApp()
const { request } = require('../../utils/request')

const fmtM = (m) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`

Page({
  data: { id: 0, type: 'private', code: '', title: '', timeText: '', status: '', canReview: false },

  onLoad(query) {
    this.setData({ id: query.id, type: query.type || 'private' })
  },

  onShow() { this.load() },

  async load() {
    const app2 = app
    await app2.ensureLogin()
    const { id, type } = this.data
    let title, timeText, status, canCode = false, canReview = false
    if (type === 'group') {
      const d = await request({ url: `/api/v1/group-classes/${id}` })
      title = d.class.title
      timeText = `${d.class.classDate} ${fmtM(d.class.startMinute)}-${fmtM(d.class.endMinute)} ${d.class.location}`
      status = d.class.status
      canCode = d.myStatus === 'enrolled'
      canReview = d.myStatus === 'verified'
    } else {
      const list = await request({ url: '/api/v1/bookings' })
      const b = list.find((x) => x.id === Number(id))
      if (!b) return
      title = '私教课'
      timeText = `${b.bookingDate} ${fmtM(b.startMinute)}-${fmtM(b.endMinute)}`
      status = b.status
      canCode = b.status === 'pending' || b.status === 'confirmed'
      canReview = b.status === 'completed'
    }
    this.setData({ title, timeText, status, canReview })
    if (canCode) {
      const url = type === 'group' ? `/api/v1/group-classes/${id}/code` : `/api/v1/bookings/${id}/code`
      const v = await request({ url })
      this.setData({ code: v.code })
    }
  },

  onCopy() {
    wx.setClipboardData({ data: this.data.code })
  },

  goReview() {
    wx.navigateTo({ url: '/pages/review/create' })
  },
})
