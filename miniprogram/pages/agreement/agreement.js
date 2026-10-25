const { request } = require('../../utils/request')

Page({
  data: { title: '', content: '' },

  onLoad(query) {
    this.type = query.type === 'user' ? 'user' : 'privacy'
    this.load()
  },

  async load() {
    // 公开接口，无需登录
    const data = await request({ url: `/api/v1/agreement?type=${this.type}` })
    wx.setNavigationBarTitle({ title: data.title })
    this.setData({ title: data.title, content: data.content })
  },
})
