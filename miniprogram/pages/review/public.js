const app = getApp()
const { request } = require('../../utils/request')

// 公开评价列表：?type=package|coach&id=X
Page({
  data: { type: 'package', id: 0, items: [], total: 0, page: 1, loading: false, done: false },

  onLoad(query) {
    this.setData({ type: query.type || 'package', id: Number(query.id) || 0 })
  },

  onShow() {
    if (!this.data.items.length) this.reload()
  },

  async reload() {
    this.setData({ items: [], page: 1, done: false })
    await this.loadMore()
  },

  onReachBottom() { this.loadMore() },

  async loadMore() {
    const { type, id, page, loading, done } = this.data
    if (loading || done || !id) return
    this.setData({ loading: true })
    try {
      const d = await request({ url: `/api/v1/${type === 'coach' ? 'coaches' : 'packages'}/${id}/reviews?page=${page}&pageSize=10` })
      const items = d.items.map((x) => ({
        ...x.review,
        nickname: (x.nickname || '学员').slice(0, 12),
        starsArr: [1, 2, 3, 4, 5].map((i) => i <= x.review.rating),
        timeText: String(x.review.createdAt).slice(0, 10),
      }))
      this.setData({
        items: this.data.items.concat(items),
        total: d.total,
        page: page + 1,
        done: this.data.items.length + items.length >= d.total,
      })
    } finally {
      this.setData({ loading: false })
    }
  },
})
