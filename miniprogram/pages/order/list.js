const app = getApp()
const { request } = require('../../utils/request')

const STATUS_TEXT = {
  pending: '待支付', paid: '已支付', closed: '已关闭',
  refunding: '退款中', refunded: '已退款',
}

Page({
  data: { list: [], page: 1, loading: false, finished: false },

  onShow() { this.setData({ page: 1, list: [], finished: false }); this.load() },
  onReachBottom() { if (!this.data.finished && !this.data.loading) this.load() },

  async load() {
    const app2 = app
    await app2.ensureLogin()
    this.setData({ loading: true })
    try {
      const data = await request({ url: `/api/v1/orders?page=${this.data.page}&page_size=10` })
      const list = this.data.page === 1 ? data.list : this.data.list.concat(data.list)
      list.forEach((o) => {
        o.statusText = STATUS_TEXT[o.status] || o.status
        o.amountYuan = (o.amount / 100).toFixed(2)
        o.couponCutYuan = o.couponAmount > 0 ? (o.couponAmount / 100).toFixed(2) : 0
        o.timeText = String(o.createdAt).replace('T', ' ').slice(0, 16)
      })
      this.setData({ list, page: this.data.page + 1, finished: list.length >= data.total })
    } finally { this.setData({ loading: false }) }
  },

  async onPay(e) {
    const id = e.currentTarget.dataset.id
    try {
      const res = await request({ url: `/api/v1/orders/${id}/prepay`, method: 'POST', data: {} })
      if (res.mockPaid) {
        wx.showToast({ title: '支付成功', icon: 'success' })
        this.setData({ page: 1, list: [], finished: false })
        this.load()
      }
    } catch (err) { /* toast 已处理 */ }
  },

  async onRefund(e) {
    const id = e.currentTarget.dataset.id
    const { confirm } = await new Promise((resolve) => {
      wx.showModal({
        title: '申请退款',
        content: '提交后由工作室审核，未开卡 N 天内全额退',
        success: resolve,
      })
    })
    if (!confirm) return
    const { value } = await new Promise((resolve) => {
      wx.showModal({
        title: '退款原因', editable: true, placeholderText: '选填',
        success: resolve, fail: () => resolve({ value: '' }),
      })
    })
    await request({ url: `/api/v1/orders/${id}/refund`, method: 'POST', data: { reason: value || '' } })
    wx.showToast({ title: '已提交审核', icon: 'success' })
    this.setData({ page: 1, list: [], finished: false })
    this.load()
  },
})
