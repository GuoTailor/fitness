const { request } = require('../../utils/request')

const WEEK = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
const fmtM = (m) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`
const todayStr = () => {
  const d = new Date()
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}
const addDays = (dateStr, days) => {
  const d = new Date(dateStr + 'T00:00:00')
  d.setDate(d.getDate() + days)
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}

Page({
  data: {
    items: [],
    saving: false,
    // 请假
    leaveDate: '',
    leaveDates: [],
    leaveStart: '09:00',
    leaveEnd: '21:00',
    leaves: [],
  },

  onLoad() {
    const t = todayStr()
    this.setData({ leaveDate: t, leaveDates: [t, addDays(t, 1), addDays(t, 2), addDays(t, 3), addDays(t, 4), addDays(t, 5), addDays(t, 6)] })
    this.load()
  },

  async load() {
    const rows = await request({ url: '/api/v1/coach/schedule' })
    // 转成 7 天 × 窗口编辑结构
    const byDay = {}
    rows.forEach((r) => {
      ;(byDay[r.weekday] = byDay[r.weekday] || []).push({
        startMinute: r.startMinute, endMinute: r.endMinute, s: fmtM(r.startMinute), e: fmtM(r.endMinute),
      })
    })
    const items = WEEK.map((label, wd) => ({ weekday: wd, label, windows: byDay[wd] || [] }))
    this.setData({ items })
    this.loadLeaves()
  },

  addWindow(e) {
    const wd = Number(e.currentTarget.dataset.wd)
    const items = this.data.items
    items[wd].windows.push({ startMinute: 540, endMinute: 1260, s: '09:00', e: '21:00' })
    this.setData({ items })
  },
  removeWindow(e) {
    const { wd, idx } = e.currentTarget.dataset
    const items = this.data.items
    items[wd].windows.splice(idx, 1)
    this.setData({ items })
  },
  onTime(e) {
    const { wd, idx, field } = e.currentTarget.dataset
    const v = e.detail.value // "HH:mm"
    const items = this.data.items
    items[wd].windows[idx][field === 's' ? 's' : 'e'] = v
    items[wd].windows[idx][field === 's' ? 'startMinute' : 'endMinute'] =
      Number(v.split(':')[0]) * 60 + Number(v.split(':')[1])
    this.setData({ items })
  },

  async onSave() {
    const items = []
    this.data.items.forEach((d) => {
      d.windows.forEach((w) => {
        items.push({ weekday: d.weekday, startMinute: w.startMinute, endMinute: w.endMinute })
      })
    })
    this.setData({ saving: true })
    try {
      await request({ url: '/api/v1/coach/schedule', method: 'PUT', data: { items } })
      wx.showToast({ title: '已保存', icon: 'success' })
    } finally { this.setData({ saving: false }) }
  },

  // ---- 请假管理 ----
  async loadLeaves() {
    try {
      const leaves = await request({ url: `/api/v1/coach/leaves?from=${todayStr()}&to=${addDays(todayStr(), 60)}` })
      leaves.forEach((l) => {
        l.rangeText = `${fmtM(l.startMinute)}-${fmtM(l.endMinute)}`
        l.fullDay = l.startMinute === 0 && l.endMinute === 1440
      })
      this.setData({ leaves })
    } catch (e) { /* toast 已处理 */ }
  },
  onLeaveDate(e) { this.setData({ leaveDate: e.detail.value }) },
  onLeaveStart(e) { this.setData({ leaveStart: e.detail.value }) },
  onLeaveEnd(e) { this.setData({ leaveEnd: e.detail.value }) },

  async onAddLeave() {
    const toMin = (s) => Number(s.split(':')[0]) * 60 + Number(s.split(':')[1])
    try {
      const res = await request({
        url: '/api/v1/coach/leaves', method: 'POST',
        data: {
          date: this.data.leaveDate,
          startMinute: toMin(this.data.leaveStart),
          endMinute: toMin(this.data.leaveEnd),
          reason: '',
        },
      })
      if (res.affectedBookings > 0) {
        wx.showModal({
          title: '已登记请假',
          content: `该时段已有 ${res.affectedBookings} 个预约，请尽快与学员协商改期`,
          showCancel: false,
        })
      } else {
        wx.showToast({ title: '已登记', icon: 'success' })
      }
      this.loadLeaves()
    } catch (e) { /* toast 已处理 */ }
  },

  async onRemoveLeave(e) {
    const id = e.currentTarget.dataset.id
    const { confirm } = await new Promise((r) => wx.showModal({ title: '撤销该请假？', success: r }))
    if (!confirm) return
    await request({ url: `/api/v1/coach/leaves/${id}`, method: 'DELETE' })
    wx.showToast({ title: '已撤销', icon: 'success' })
    this.loadLeaves()
  },
})
