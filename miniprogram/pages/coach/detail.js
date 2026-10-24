const { request } = require('../../utils/request')

Page({
  data: { id: 0, detail: null },

  onLoad(query) {
    this.setData({ id: query.id })
    this.load()
  },

  async load() {
    const data = await request({ url: `/api/v1/coaches/${this.data.id}` })
    data.packages.forEach((p) => { p.priceYuan = (p.minPrice / 100).toFixed(0) })
    // 时段模板转可读文本（M3 接入实时可约日历）
    data.scheduleText = (data.schedules || []).map((s) =>
      `周${'日一二三四五六'[s.weekday]} ${fmtMinute(s.startMinute)}-${fmtMinute(s.endMinute)}`).join('　')
    this.setData({ detail: data })
  },

  onPkg(e) {
    wx.navigateTo({ url: '/pages/course/detail?id=' + e.currentTarget.dataset.id })
  },

  onBook() {
    const { detail } = this.data
    wx.navigateTo({
      url: `/pages/booking/private?coachId=${detail.coach.id}&coachName=${encodeURIComponent(detail.coach.name)}`,
    })
  },
})

function fmtMinute(m) {
  const h = Math.floor(m / 60), mm = m % 60
  return `${h}:${String(mm).padStart(2, '0')}`
}
