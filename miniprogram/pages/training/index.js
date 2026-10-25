const app = getApp()
const { request } = require('../../utils/request')

const STATUS_TEXT = {
  inactive: '未开卡（首次上课自动激活）', active: '生效中', exhausted: '已用完',
  expired: '已过期', refunded: '已退款',
}

Page({
  data: {
    tab: 'hours', // hours=我的课时 training=训练跟踪
    list: [], txns: [], current: null,
    stats: null, records: [], pending: [],
    month: '', calendar: [],
    // 体重
    weightKg: '', latestWeight: '', weightBars: [], weightCount: 0,
    // 教练反馈
    feedbacks: [],
  },

  onShow() { this.load() },

  async load() {
    await app.ensureLogin()
    this.loadHours()
    this.loadTraining()
    this.loadWeight()
    this.loadFeedbacks()
  },

  async loadHours() {
    const list = await request({ url: '/api/v1/member/packages' })
    list.forEach((p) => {
      p.statusText = STATUS_TEXT[p.status] || p.status
      p.remaining = p.totalSessions - p.usedSessions
      p.percent = p.totalSessions ? Math.round((p.usedSessions / p.totalSessions) * 100) : 0
      p.expireText = p.expireDate ? String(p.expireDate).slice(0, 10) : '—'
    })
    this.setData({ list })
  },

  async loadTraining() {
    const [tr, pend] = await Promise.all([
      request({ url: '/api/v1/training' }),
      request({ url: '/api/v1/reviews/reviewable' }),
    ])
    const records = tr.records || []
    records.forEach((r) => { r.sourceText = r.source === 'group' ? '团课' : '私教' })
    pend.forEach((x) => { x.dateText = String(x.verifiedAt || '').slice(0, 10) })
    this.setData({ stats: tr.stats, records, pending: pend })
    this.loadCalendar()
  },

  async loadCalendar() {
    const d = new Date()
    const month = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
    const cal = await request({ url: `/api/v1/training/calendar?month=${month}` })
    const map = {}
    cal.days.forEach((x) => { map[x.date] = x.count })
    const first = new Date(d.getFullYear(), d.getMonth(), 1)
    const daysInMonth = new Date(d.getFullYear(), d.getMonth() + 1, 0).getDate()
    const cells = []
    for (let i = 0; i < first.getDay(); i++) cells.push({ blank: 1 })
    const today = `${month}-${String(d.getDate()).padStart(2, '0')}`
    for (let i = 1; i <= daysInMonth; i++) {
      const ds = `${month}-${String(i).padStart(2, '0')}`
      cells.push({ day: i, date: ds, count: map[ds] || 0, today: ds === today })
    }
    this.setData({ month, calendar: cells })
  },

  onTab(e) { this.setData({ tab: e.currentTarget.dataset.tab }) },

  // ---- 体重记录 ----
  async loadWeight() {
    try {
      const rows = await request({ url: '/api/v1/body-metrics?limit=30' })
      const bars = []
      if (rows.length) {
        const vals = rows.map((r) => r.weightG)
        const min = Math.min(...vals), max = Math.max(...vals)
        const span = Math.max(max - min, 1)
        rows.forEach((r) => {
          bars.push({ pct: Math.round(15 + ((r.weightG - min) / span) * 85), date: String(r.recordDate).slice(5) })
        })
      }
      const last = rows.length ? (rows[rows.length - 1].weightG / 1000).toFixed(1) : ''
      this.setData({ weightBars: bars, weightCount: rows.length, latestWeight: last })
    } catch (e) { /* 静默 */ }
  },

  onWeightInput(e) { this.setData({ weightKg: e.detail.value }) },

  async onSaveWeight() {
    const kg = parseFloat(this.data.weightKg)
    if (!kg || kg < 20 || kg > 300) return wx.showToast({ title: '请输入 20~300 之间的体重', icon: 'none' })
    const d = new Date()
    const date = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    try {
      await request({ url: '/api/v1/body-metrics', method: 'POST', data: { date, weightG: Math.round(kg * 100) } })
      wx.showToast({ title: '已记录', icon: 'success' })
      this.setData({ weightKg: '' })
      this.loadWeight()
    } catch (e) { /* toast 已处理 */ }
  },

  // ---- 教练课后反馈 ----
  async loadFeedbacks() {
    try {
      const rows = await request({ url: '/api/v1/feedback/mine' })
      rows.forEach((x) => {
        x.dateText = String((x.feedback && x.feedback.createdAt) || '').replace('T', ' ').slice(0, 16)
        x.typeText = x.bizType === 'group' ? '团课' : '私教'
      })
      this.setData({ feedbacks: rows })
    } catch (e) { /* 静默 */ }
  },

  goReport() { wx.navigateTo({ url: '/pages/training/report' }) },

  async onPkg(e) {
    const id = Number(e.currentTarget.dataset.id)
    if (this.data.current === id) {
      this.setData({ current: null, txns: [] })
      return
    }
    const txns = await request({ url: `/api/v1/member/packages/${id}/transactions` })
    txns.forEach((t) => {
      t.sign = t.sessionsChange > 0 ? '+' : ''
      t.timeText = String(t.createdAt).replace('T', ' ').slice(0, 16)
    })
    this.setData({ current: id, txns })
  },

  goReview() { wx.navigateTo({ url: '/pages/review/create' }) },
  goBuy() { wx.switchTab({ url: '/pages/course/list' }) },
})
