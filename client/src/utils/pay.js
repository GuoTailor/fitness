// 支付封装：
// - 小程序端：prepay(jsapi) → uni.requestPayment
// - H5 端：prepay(channel=native) → 二维码弹层 + 轮询订单状态
// mock 模式（pay.mock=true）下 prepay 直接返回 mockPaid，两端都表现为「支付成功」
import { request } from './request'

// qrcode 包仅 H5 端使用（条件编译，MP 构建不打包）
// #ifdef H5
import QR from 'qrcode'
// #endif

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/**
 * 支付订单
 * @param {number} orderId
 * @param {{onQrCode?: (dataUrl: string) => void, onCancel?: () => void}} opts
 *   H5 真实通道时通过 onQrCode 把二维码 dataURL 交给调用方弹层展示
 * @returns {Promise<{paid: boolean, status?: string, timeout?: boolean}>}
 */
export async function payOrder(orderId, opts = {}) {
  // #ifdef MP-WEIXIN
  const res = await request({ url: `/api/v1/orders/${orderId}/prepay`, method: 'POST', data: {} })
  if (res.mockPaid) return { paid: true }
  await new Promise((resolve, reject) => {
    uni.requestPayment({
      provider: 'wxpay',
      timeStamp: res.payParams.timeStamp,
      nonceStr: res.payParams.nonceStr,
      package: res.payParams.package,
      signType: res.payParams.signType,
      paySign: res.payParams.paySign,
      success: resolve,
      fail: (e) => reject(new Error(e.errMsg || '支付未完成'))
    })
  })
  return { paid: true }
  // #endif

  // #ifdef H5
  const res = await request({
    url: `/api/v1/orders/${orderId}/prepay`,
    method: 'POST',
    data: { channel: 'native' }
  })
  if (res.mockPaid) return { paid: true }
  if (!res.codeUrl) throw new Error('支付二维码获取失败')
  const dataUrl = await QR.toDataURL(res.codeUrl, { width: 240, margin: 1 })
  if (opts.onQrCode) opts.onQrCode(dataUrl)
  // 轮询订单状态（最多 2 分钟；用户取消支付后订单仍是 pending → 超时退出）
  const deadline = Date.now() + 120000
  while (Date.now() < deadline) {
    await sleep(2500)
    const order = await request({ url: `/api/v1/orders/${orderId}`, silent: true })
    if (order.status === 'paid') return { paid: true }
    if (['closed', 'refunding', 'refunded'].includes(order.status)) {
      return { paid: false, status: order.status }
    }
  }
  return { paid: false, timeout: true }
  // #endif
}
