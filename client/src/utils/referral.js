// 老带新裂变：落地捕获 inviter → 登录后绑定（服务端幂等，失败静默）
import { request } from './request'

/** 落地时暂存邀请人（分享链接带 ?inviter=<uid>） */
export function captureInvite(options) {
  let inviter = ''
  // #ifdef H5
  const q = new URLSearchParams(window.location.search)
  inviter = q.get('inviter') || ''
  // #endif
  // #ifdef MP-WEIXIN
  const query = (options && options.query) || {}
  inviter = query.inviter || ''
  // #endif
  const id = parseInt(inviter, 10)
  if (!id || id <= 0) return
  if (uni.getStorageSync('referralBound')) return
  uni.setStorageSync('pendingInviter', id)
}

/** 登录后尝试绑定（登录成功后由 auth store 调用；失败静默清理） */
export async function tryBindInvite() {
  const id = uni.getStorageSync('pendingInviter')
  if (!id) return
  try {
    await request({ url: '/api/v1/referral/bind', method: 'POST', data: { inviter: id } })
    uni.showToast({ title: '已接受好友邀请', icon: 'none' })
  } catch (e) {
    // 已绑定/不符合条件：静默
  } finally {
    uni.removeStorageSync('pendingInviter')
    uni.setStorageSync('referralBound', 1)
  }
}

/** 分享标题（带邀请语义） */
export function shareTitle() {
  return '一起来健身，约课更方便'
}

/**
 * 小程序分享卡片参数（onShareAppMessage 返回值用）
 * @param uid 当前用户 id（未登录传 0，不带邀请参）
 */
export function mpSharePayload(pagePath, uid) {
  const extra = uid ? '?inviter=' + uid : ''
  return { title: shareTitle(), path: pagePath + extra }
}

/** H5 分享/复制邀请链接（uid 同上） */
export function shareLink(pagePath, uid) {
  const extra = uid ? '?inviter=' + uid : ''
  const link = window.location.origin + '/' + pagePath.replace(/^\//, '') + extra
  if (navigator.share) {
    navigator.share({ title: shareTitle(), url: link }).catch(() => copyText(link))
  } else {
    copyText(link)
  }
}

function copyText(text) {
  uni.setClipboardData({
    data: text,
    success: () => uni.showToast({ title: '链接已复制', icon: 'none' })
  })
}
