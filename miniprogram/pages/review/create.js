const app = getApp()
const { request } = require('../../utils/request')

Page({
  data: {
    pending: [],      // 待评价列表（无 verificationId 时展示选择）
    current: null,    // 正在评价的课程
    pkgStars: 5,
    coachStars: 5,
    content: '',
    submitting: false,
  },

  onLoad(query) {
    this.vid = Number(query.verificationId) || 0
  },

  onShow() { this.load() },

  async load() {
    await app.ensureLogin()
    const items = await request({ url: '/api/v1/reviews/reviewable' })
    items.forEach((x) => {
      x.dateText = String(x.verifiedAt || '').slice(0, 10)
      x.typeText = x.bizType === 'group' ? '团课' : '私教'
    })
    if (this.vid) {
      const cur = items.find((x) => x.verificationId === this.vid)
      if (cur) this.setData({ current: cur, pending: [] })
      else this.setData({ current: null, pending: [] }) // 已被评价过
    } else {
      this.setData({ pending: items, current: items.length === 1 ? items[0] : null })
    }
  },

  onPick(e) {
    const item = this.data.pending[Number(e.currentTarget.dataset.index)]
    this.setData({ current: item })
  },

  setStars(e) {
    const field = e.currentTarget.dataset.field
    this.setData({ [field]: Number(e.currentTarget.dataset.val) })
  },

  onInput(e) {
    this.setData({ content: e.detail.value })
  },

  async onSubmit() {
    if (this.data.submitting) return
    const cur = this.data.current
    if (!cur) return
    this.setData({ submitting: true })
    try {
      await request({
        url: '/api/v1/reviews',
        method: 'POST',
        data: {
          verificationId: cur.verificationId,
          packageRating: this.data.pkgStars,
          coachRating: this.data.coachStars,
          content: this.data.content,
        },
      })
      wx.showToast({ title: '评价成功，感谢反馈', icon: 'success' })
      setTimeout(() => wx.navigateBack({ fail: () => wx.switchTab({ url: '/pages/mine/mine' }) }), 800)
    } catch (e) {
      // request 已 toast
    } finally {
      this.setData({ submitting: false })
    }
  },
})
