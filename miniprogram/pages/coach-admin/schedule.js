const { request } = require('../../utils/request')

const WEEK = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
const fmtM = (m) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`

Page({
  data: { items: [], saving: false },

  onLoad() { this.load() },

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
})
