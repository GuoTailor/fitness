const app = getApp()
const { request } = require('../../utils/request')
const { requestSubscribe } = require('../../utils/subscribe')

const fmtM = (m) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`
const addDays = (d, n) => {
  const t = new Date(d.getTime() + n * 86400000)
  return t
}
const iso = (d) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
const WEEK = ['日', '一', '二', '三', '四', '五', '六']

Page({
  data: {
    coachId: 0,
    coachName: '',
    days: [],           // 未来 7 天
    activeDate: '',
    slots: [],
    selectedSlot: -1,
    mps: [],            // 可用课包（该教练在售）
    selectedMp: -1,
    submitting: false,
  },

  onLoad(query) {
    const days = []
    for (let i = 1; i <= 7; i++) {
      const d = addDays(new Date(), i)
      days.push({ date: iso(d), label: `周${WEEK[d.getDay()]}`, sub: `${d.getMonth() + 1}/${d.getDate()}` })
    }
    this.setData({ coachId: Number(query.coachId || 0), coachName: decodeURIComponent(query.coachName || ''), days, activeDate: days[0].date })
    this.loadSlots(days[0].date)
    this.loadPackages()
  },

  onDate(e) {
    const date = e.currentTarget.dataset.date
    this.setData({ activeDate: date, selectedSlot: -1 })
    this.loadSlots(date)
  },
  onSlot(e) {
    this.setData({ selectedSlot: Number(e.currentTarget.dataset.index) })
  },
  onMp(e) {
    this.setData({ selectedMp: Number(e.currentTarget.dataset.index) })
  },

  async loadSlots(date) {
    const data = await request({ url: `/api/v1/coaches/${this.data.coachId}/slots?date=${date}` })
    this.setData({ slots: data.slots || [] })
  },

  async loadPackages() {
    const app2 = app
    await app2.ensureLogin()
    const list = await request({ url: '/api/v1/member/packages' })
    const usable = list.filter((p) => p.status === 'active' || p.status === 'inactive')
    this.setData({ mps: usable, selectedMp: usable.length ? 0 : -1 })
  },

  async onSubmit() {
    const { slots, selectedSlot, mps, selectedMp, activeDate, coachId, submitting } = this.data
    if (submitting) return
    if (selectedSlot < 0) return wx.showToast({ title: '请选择时间', icon: 'none' })
    if (selectedMp < 0) return wx.showToast({ title: '暂无可用课包，请先购买', icon: 'none' })
    this.setData({ submitting: true })
    try {
      await request({
        url: '/api/v1/bookings', method: 'POST',
        data: {
          coachId, memberPackageId: mps[selectedMp].id,
          date: activeDate, startMinute: slots[selectedSlot].startMinute,
        },
      })
      requestSubscribe(['booking_new', 'booking_confirmed']) // 授权订阅：预约与确认通知
      wx.showModal({
        title: '预约成功', content: '已扣 1 课时，等待教练确认',
        confirmText: '看预约', cancelText: '返回',
        success: (r) => { if (r.confirm) wx.navigateTo({ url: '/pages/booking/list' }) },
      })
      this.loadPackages()
    } catch (e) { /* toast 已处理 */ } finally { this.setData({ submitting: false }) }
  },
})
