const { request } = require('../../utils/request')

const CATEGORIES = [
  { value: '', label: '全部' },
  { value: 'private', label: '私教 1v1' },
  { value: 'group', label: '团课卡' },
  { value: 'trial', label: '体验课' },
]
const TAGS = ['', '减脂', '增肌', '塑形', '康复']

Page({
  data: {
    categories: CATEGORIES,
    tags: TAGS,
    activeCategory: '',
    activeTag: '',
    keyword: '',
    list: [],
    total: 0,
    page: 1,
    loading: false,
    finished: false,
  },

  onLoad(query) {
    if (query.category) this.setData({ activeCategory: query.category })
    this.load()
  },

  onPullDownRefresh() {
    this.setData({ page: 1, finished: false })
    this.load().then(() => wx.stopPullDownRefresh())
  },

  onReachBottom() {
    if (!this.data.finished && !this.data.loading) this.load()
  },

  onCategory(e) {
    this.setData({ activeCategory: e.currentTarget.dataset.value, page: 1, finished: false })
    this.load()
  },
  onTag(e) {
    this.setData({ activeTag: e.currentTarget.dataset.value, page: 1, finished: false })
    this.load()
  },
  onKeyword(e) {
    this.setData({ keyword: e.detail.value })
  },
  onSearch() {
    this.setData({ page: 1, finished: false })
    this.load()
  },

  async load() {
    const { activeCategory: category, activeTag: tag, keyword, page } = this.data
    this.setData({ loading: true })
    try {
      const data = await request({
        url: `/api/v1/packages?page=${page}&page_size=10&category=${category}&tag=${tag}&keyword=${keyword}`,
      })
      const list = page === 1 ? data.list : this.data.list.concat(data.list)
      this.setData({ list, total: data.total, page: page + 1, finished: list.length >= data.total })
    } finally {
      this.setData({ loading: false })
    }
  },

  onPkgTap(e) {
    wx.navigateTo({ url: '/pages/course/detail?id=' + e.currentTarget.dataset.id })
  },
})
