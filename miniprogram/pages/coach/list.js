const { request } = require('../../utils/request')

Page({
  data: {
    list: [], total: 0, page: 1, keyword: '', loading: false, finished: false,
    ratingChips: [
      { label: '全部', value: 0 },
      { label: '4.0分以上', value: 4 },
      { label: '4.5分以上', value: 4.5 },
    ],
    minRating: 0,
  },

  onLoad() { this.load() },
  onReachBottom() { if (!this.data.finished && !this.data.loading) this.load() },

  onKeyword(e) { this.setData({ keyword: e.detail.value }) },
  onSearch() { this.setData({ page: 1, finished: false, list: [] }); this.load() },

  onRatingChip(e) {
    const v = Number(e.currentTarget.dataset.value)
    this.setData({ minRating: v, page: 1, finished: false, list: [] })
    this.load()
  },

  async load() {
    this.setData({ loading: true })
    try {
      const { page, keyword, minRating } = this.data
      const data = await request({ url: `/api/v1/coaches?page=${page}&page_size=10&keyword=${keyword}&minRating=${minRating}` })
      const list = page === 1 ? data.list : this.data.list.concat(data.list)
      this.setData({ list, total: data.total, page: page + 1, finished: list.length >= data.total })
    } finally { this.setData({ loading: false }) }
  },

  onCoach(e) { wx.navigateTo({ url: '/pages/coach/detail?id=' + e.currentTarget.dataset.id }) },
  goApply() { wx.navigateTo({ url: '/pages/coach/apply' }) },
})
