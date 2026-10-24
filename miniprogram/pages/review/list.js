const app = getApp()
const { request } = require('../../utils/request')

const STATUS = { pending: '审核中', approved: '已通过', hidden: '已隐藏' }

Page({
  data: { items: [] },

  onShow() { this.load() },

  async load() {
    await app.ensureLogin()
    const items = await request({ url: '/api/v1/reviews/mine' })
    items.forEach((x) => {
      const r = x.review
      x.statusText = STATUS[r.status] || r.status
      x.typeText = x.targetType === 'coach' ? '教练服务' : '课程体验'
      x.timeText = String(r.createdAt).replace('T', ' ').slice(0, 16)
      x.starsArr = [1, 2, 3, 4, 5].map((i) => i <= r.rating)
    })
    this.setData({ items })
  },
})
