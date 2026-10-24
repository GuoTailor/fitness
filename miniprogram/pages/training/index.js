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
  },

  onShow() { this.load() },

  async load() {
    await app.ensureLogin()
    this.loadHours()
    this.loadTraining()
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
