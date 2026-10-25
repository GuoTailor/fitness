// 下单后发起支付：mock 通道直接成功；真实通道拉起 wx.requestPayment
const { request } = require('./request')

async function payOrder(orderId) {
  const payRes = await request({ url: `/api/v1/orders/${orderId}/prepay`, method: 'POST', data: {} })
  if (payRes.mockPaid) return { mockPaid: true }
  if (payRes.payParams) {
    const p = payRes.payParams
    await new Promise((resolve, reject) => {
      wx.requestPayment({
        timeStamp: p.timeStamp, nonceStr: p.nonceStr, package: p.package,
        signType: p.signType, paySign: p.paySign,
        success: resolve, fail: reject,
      })
    })
    return { paid: true }
  }
  return {}
}

module.exports = { payOrder }
