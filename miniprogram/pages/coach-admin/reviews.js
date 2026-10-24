const app = getApp()
const { request } = require('../../utils/request')

const STATUS = { pending: '审核中', approved: '已通过', hidden: '已隐藏' }

Page({
  data: { items: [] },

  onShow() { this.load() },

  async load() {
    await app.ensureLogin()
    const items = await request({ url: '/api/v1/coach/reviews' })
    items.forEach((x) => {
      x.statusText = STATUS[x.review.status] || x.review.status
      x.nickname = (x.nickname || '学员').slice(0, 12)
      x.timeText = String(x.review.createdAt).slice(0, 10)
      x.starsArr = [1, 2, 3, 4, 5].map((i) => i <= x.review.rating)
    })
    this.setData({ items })
  },

  async onReply(e) {
    const id = Number(e.currentTarget.dataset.id)
    const { confirm, content } = await new Promise((resolve) => {
      wx.showModal({
        title: '回复评价',
        editable: true,
        placeholderText: '感谢您的反馈…（200 字内）',
        success: resolve,
        fail: () => resolve({ confirm: false }),
      })
    })
    if (!confirm || !content || !content.trim()) return
    await request({ url: `/api/v1/coach/reviews/${id}/reply`, method: 'POST', data: { content: content.trim() } })
    wx.showToast({ title: '已回复', icon: 'success' })
    this.load()
  },
})
