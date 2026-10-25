const { request } = require('../../utils/request')

const fmtM = (m) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`
const tomorrow = () => {
  const d = new Date(Date.now() + 86400000)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

Page({
  data: { list: [], pkgs: [], pkgText: '', showForm: false, form: null },

  onShow() { this.load() },

  async load() {
    const [list, pkgs] = await Promise.all([
      request({ url: '/api/v1/coach/group-classes' }),
      request({ url: '/api/v1/coach/packages' }),
    ])
    list.forEach((g) => { g.timeText = `${g.classDate} ${fmtM(g.startMinute)}` })
    this.setData({ list, pkgs })
    this.syncPkgText()
  },

  // 选中课包的展示文案：WXML 表达式不支持箭头函数/方法调用，需在 JS 里算好
  syncPkgText() {
    const { pkgs, form } = this.data
    if (!pkgs.length) return this.setData({ pkgText: '无可售课包' })
    const cur = pkgs.find((p) => form && p.id === form.packageId)
    this.setData({ pkgText: cur ? cur.title : '请选择' })
  },

  openForm() {
    this.setData({
      showForm: true,
      form: {
        packageId: this.data.pkgs.length ? this.data.pkgs[0].id : 0,
        title: '', date: tomorrow(), startMinute: 1140, endMinute: 1200,
        location: '', capacity: 10,
        s: '19:00', e: '20:00',
      },
    })
    this.syncPkgText()
  },
  onInput(e) {
    const f = this.data.form
    f[e.currentTarget.dataset.field] = e.detail.value
    this.setData({ form: f })
  },
  onPkg(e) {
    // picker 的 detail.value 是 range 下标，映射回课包 id
    const f = this.data.form
    f.packageId = (this.data.pkgs[Number(e.detail.value)] || {}).id || 0
    this.setData({ form: f })
    this.syncPkgText()
  },
  onTime(e) {
    const f = this.data.form
    const field = e.currentTarget.dataset.field
    f[field] = e.detail.value
    f[field === 's' ? 'startMinute' : 'endMinute'] = Number(e.detail.value.split(':')[0]) * 60 + Number(e.detail.value.split(':')[1])
    this.setData({ form: f })
  },

  async onPublish() {
    const f = this.data.form
    if (!f.title) return wx.showToast({ title: '请填标题', icon: 'none' })
    await request({ url: '/api/v1/coach/group-classes', method: 'POST', data: {
      packageId: f.packageId, title: f.title, date: f.date,
      startMinute: f.startMinute, endMinute: f.endMinute,
      location: f.location, capacity: Number(f.capacity),
    } })
    wx.showToast({ title: '已发布', icon: 'success' })
    this.setData({ showForm: false })
    this.load()
  },

  async onEnrollments(e) {
    const id = e.currentTarget.dataset.id
    const list = await request({ url: `/api/v1/coach/group-classes/${id}/enrollments` })
    const names = list.map((x) => `${x.userName || '学员'}（${x.userPhone || '无手机号'}）${x.enrollment.status === 'verified' ? '✓' : ''}`)
    wx.showModal({ title: '报名名单', content: names.join('\n') || '暂无报名', showCancel: false })
  },

  async onCancel(e) {
    const id = e.currentTarget.dataset.id
    const { confirm } = await new Promise((r) =>
      wx.showModal({ title: '取消团课', content: '将退还全部已报名学员的课时', success: r }))
    if (!confirm) return
    await request({ url: `/api/v1/coach/group-classes/${id}/cancel`, method: 'POST', data: { reason: '教练取消' } })
    this.load()
  },
})
