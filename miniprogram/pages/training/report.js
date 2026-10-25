const app = getApp()
const { request } = require('../../utils/request')

const iso = (d) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`

Page({
  data: { date: '', weekText: '', report: null },

  onLoad() { this.setData({ date: iso(new Date()) }); this.load() },

  shiftWeek(days) {
    const d = new Date(this.data.date + 'T00:00:00')
    d.setDate(d.getDate() + days)
    this.setData({ date: iso(d) })
    this.load()
  },
  onPrev() { this.shiftWeek(-7) },
  onNext() { this.shiftWeek(7) },

  async load() {
    await app.ensureLogin()
    const rep = await request({ url: `/api/v1/training/report?date=${this.data.date}` })
    rep.current = rep.current || {}
    rep.prev = rep.prev || {}
    rep.byCoach = rep.byCoach || []
    const delta = (cur, prev, key) => {
      const c = cur[key] || 0, p = prev[key] || 0
      if (p === 0) return c > 0 ? '新纪录' : '—'
      const pct = Math.round(((c - p) / p) * 100)
      return (pct > 0 ? '+' : '') + pct + '%'
    }
    rep.deltas = {
      sessions: delta(rep.current, rep.prev, 'sessions'),
      minutes: delta(rep.current, rep.prev, 'minutes'),
      activeDays: delta(rep.current, rep.prev, 'activeDays'),
    }
    if (rep.weight && rep.weight.deltaKg !== null && rep.weight.deltaKg !== undefined) {
      const dkg = rep.weight.deltaKg
      rep.weight.deltaText = (dkg > 0 ? '+' : '') + dkg.toFixed(1) + 'kg'
    }
    this.setData({ report: rep, weekText: `${rep.weekStart} ~ ${rep.weekEnd}` })
  },
})
