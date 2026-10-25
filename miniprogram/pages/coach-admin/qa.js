const { request } = require('../../utils/request')

Page({
  data: { list: [], editing: null, content: '' },

  onShow() { this.load() },

  async load() {
    const rows = await request({ url: '/api/v1/coach/qa' })
    rows.forEach((q) => {
      q.timeText = String(q.createdAt || '').replace('T', ' ').slice(0, 16)
      ;(q.answers || []).forEach((a) => {
        a.timeText = String(a.createdAt || '').replace('T', ' ').slice(0, 16)
      })
    })
    this.setData({ list: rows })
  },

  async onReply(e) {
    this.setData({ editing: Number(e.currentTarget.dataset.id), content: '' })
  },
  onInput(e) { this.setData({ content: e.detail.value }) },
  onCancel() { this.setData({ editing: null, content: '' }) },

  async onSave() {
    if (!this.data.content.trim()) return wx.showToast({ title: '请填写回复', icon: 'none' })
    try {
      await request({
        url: `/api/v1/coach/qa/${this.data.editing}/answers`, method: 'POST',
        data: { content: this.data.content.trim() },
      })
      wx.showToast({ title: '已回复', icon: 'success' })
      this.setData({ editing: null, content: '' })
      this.load()
    } catch (e) { /* toast 已处理 */ }
  },

  async onPin(e) {
    const id = e.currentTarget.dataset.id
    await request({ url: `/api/v1/coach/qa/${id}/pin`, method: 'POST' })
    wx.showToast({ title: '已切换置顶', icon: 'none' })
    this.load()
  },
})
