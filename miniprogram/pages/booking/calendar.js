const app = getApp()
const { request } = require('../../utils/request')
const { requestSubscribe } = require('../../utils/subscribe')

const iso = (d) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
const fmtM = (m) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`

Page({
  data: { days: [], activeDate: '', classes: [], mps: [], selectedMp: -1 },

  onLoad() {
    const days = []
    for (let i = 0; i < 14; i++) days.push(iso(new Date(Date.now() + i * 86400000)))
    this.setData({ days, activeDate: days[0] })
  },

  onShow() {
    this.loadPkgs()
    this.load()
  },

  async loadPkgs() {
    const app2 = app
    await app2.ensureLogin()
    const list = await request({ url: '/api/v1/member/packages' })
    const usable = list.filter((p) => p.status === 'active' || p.status === 'inactive')
    this.setData({ mps: usable, selectedMp: usable.length ? 0 : -1 })
  },

  async load() {
    const from = this.data.days[0]
    const to = this.data.days[this.data.days.length - 1]
    const data = await request({ url: `/api/v1/group-classes?from=${from}&to=${to}` })
    const byDate = {}
    data.forEach((row) => {
      row.timeText = fmtM(row.class.startMinute)
      row.left = row.class.capacity - row.class.enrolledCount
      ;(byDate[row.class.classDate] = byDate[row.class.classDate] || []).push(row)
    })
    this.setData({ classes: byDate })
  },

  onDay(e) { this.setData({ activeDate: e.currentTarget.dataset.date }) },

  async onEnroll(e) {
    const id = e.currentTarget.dataset.id
    const { mps, selectedMp } = this.data
    if (selectedMp < 0) return wx.showToast({ title: '请先购买对应课包', icon: 'none' })
    const mp = mps.find((m) => m.packageId === Number(e.currentTarget.dataset.pkg)) || mps[selectedMp]
    try {
      await request({ url: `/api/v1/group-classes/${id}/enroll`, method: 'POST', data: { memberPackageId: mp.id } })
      requestSubscribe(['booking_confirmed']) // 授权订阅：报名结果通知
      wx.showToast({ title: '报名成功', icon: 'success' })
      this.load()
      this.loadPkgs()
    } catch (err) { /* toast 已处理 */ }
  },

  async onQuit(e) {
    const id = e.currentTarget.dataset.id
    await request({ url: `/api/v1/group-classes/${id}/quit`, method: 'POST' })
    wx.showToast({ title: '已退出，课时已返还', icon: 'none' })
    this.load()
    this.loadPkgs()
  },

  onCode(e) {
    wx.navigateTo({ url: `/pages/booking/detail?type=group&id=${e.currentTarget.dataset.id}` })
  },
})
