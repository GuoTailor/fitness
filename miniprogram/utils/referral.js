// 老带新裂变：落地页捕获 inviter 参数 → 登录后绑定（服务端幂等，绑定失败静默清理）
const { request } = require('./request')

/** 落地时暂存邀请人（分享卡片 path 带 ?inviter=<uid>） */
function captureInvite(inviter) {
  const id = parseInt(inviter, 10)
  if (!id || id <= 0) return
  if (wx.getStorageSync('referralBound')) return // 绑定过任何人就不再处理
  wx.setStorageSync('pendingInviter', id)
}

/** 登录后尝试绑定（进首页/我的页时调用；未登录会先静默 wx.login） */
async function tryBindInvite() {
  const id = wx.getStorageSync('pendingInviter')
  if (!id) return
  try {
    await getApp().ensureLogin()
    await request({ url: '/api/v1/referral/bind', method: 'POST', data: { inviter: id } })
    wx.showToast({ title: '已接受好友邀请', icon: 'none' })
  } catch (e) { /* 已绑定/不符合条件：静默 */ } finally {
    wx.removeStorageSync('pendingInviter')
    wx.setStorageSync('referralBound', 1)
  }
}

/** 分享卡片默认文案与路径（带邀请码，未登录不带参） */
function shareCard(title, basePage) {
  const app = getApp()
  const uid = app.globalData.user && app.globalData.user.id
  const extra = uid ? '?inviter=' + uid : ''
  return { title: title || '一起来健身，约课更方便', path: basePage + extra }
}

module.exports = { captureInvite, tryBindInvite, shareCard }
