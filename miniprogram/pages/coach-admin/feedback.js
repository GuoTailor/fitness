const { request } = require('../../utils/request')

Page({
  data: { list: [], editing: null, content: '' },

  onShow() { this.load() },

  async load() {
    const rows = await request({ url: '/api/v1/coach/feedbacks' })
    rows.forEach((x) => {
      x.dateText = String(x.verifiedAt || '').replace('T', ' ').slice(0, 16)
      x.typeText = x.bizType === 'group' ? '团课' : '私教'
      x.written = !!(x.feedback && x.feedback.content)
    })
    this.setData({ list: rows })
  },

  async onWrite(e) {
    const id = Number(e.currentTarget.dataset.id)
    const item = this.data.list.find((x) => x.verificationId === id)
    this.setData({ editing: id, content: (item && item.feedback && item.feedback.content) || '' })
  },

  onInput(e) { this.setData({ content: e.detail.value }) },

  async onSave() {
    if (!this.data.content.trim()) return wx.showToast({ title: '请填写反馈内容', icon: 'none' })
    try {
      await request({
        url: '/api/v1/coach/feedback', method: 'POST',
        data: { verificationId: this.data.editing, content: this.data.content },
      })
      wx.showToast({ title: '已保存', icon: 'success' })
      this.setData({ editing: null, content: '' })
      this.load()
    } catch (e) { /* toast 已处理 */ }
  },

  onCancel() { this.setData({ editing: null, content: '' }) },
})
