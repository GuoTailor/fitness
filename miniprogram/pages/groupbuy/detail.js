// 拼团详情（分享落地页）：进度 + 参团支付 + 我的订单状态
const app = getApp()
const { request } = require('../../utils/request')
const { payOrder } = require('../../utils/pay')
const referral = require('../../utils/referral')

Page({
  data: { gbid: 0, gb: null, joining: false },

  onLoad(query) {
    this.setData({ gbid: parseInt(query.gbid || 0, 10) })
  },

  onShow() {
    this.load()
    referral.tryBindInvite()
  },

  // 分享卡片带上团 ID 与邀请码（参团即裂变落地方）
  onShareAppMessage() {
    return referral.shareCard(
      '拼团更划算，一起来练！',
      '/pages/groupbuy/detail?gbid=' + this.data.gbid
    )
  },

  async load() {
    await app.ensureLogin()
    try {
      const gb = await request({ url: `/api/v1/group-buys/${this.data.gbid}` })
      gb.priceYuan = (gb.price / 100).toFixed(2)
      gb.endText = String(gb.expireAt).replace('T', ' ').slice(5, 16)
      gb.canJoin = gb.status === 1 && !gb.joined
      this.setData({ gb })
    } catch (e) { /* request 已 toast */ }
  },

  async onJoin() {
    this.setData({ joining: true })
    try {
      const order = await request({
        url: `/api/v1/group-buys/${this.data.gbid}/join`, method: 'POST', data: {},
      })
      const payRes = await payOrder(order.id)
      if (payRes.mockPaid || payRes.paid) {
        wx.showToast({ title: '参团成功，等好友上车', icon: 'none' })
      }
      this.load()
    } catch (e) { /* toast 已处理 */ } finally {
      this.setData({ joining: false })
    }
  },

  // 未支付订单继续支付
  async onPayAgain() {
    const gb = this.data.gb
    if (!gb || !gb.myOrderId) return
    try {
      const payRes = await payOrder(gb.myOrderId)
      if (payRes.mockPaid || payRes.paid) {
        wx.showToast({ title: '支付成功', icon: 'success' })
      }
      this.load()
    } catch (e) { /* toast 已处理 */ }
  },
})
