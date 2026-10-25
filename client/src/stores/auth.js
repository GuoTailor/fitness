// 鉴权 store：token/用户/教练身份
// - 小程序端：静默 wx.login 自动登录（与原小程序行为一致）
// - H5 端：未登录访问需登录功能时跳转登录页（手机号+短信验证码）
import { defineStore } from 'pinia'
import { request, setToken, getToken, onUnauthorized } from '@/utils/request'
import { tryBindInvite } from '@/utils/referral'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: getToken(),
    user: null,
    isCoach: false,
    coachId: 0
  }),
  getters: {
    loggedIn: (s) => !!s.token
  },
  actions: {
    init() {
      onUnauthorized(() => {
        this.token = ''
        this.user = null
        this.isCoach = false
        this.coachId = 0
      })
      if (this.token) {
        this.refreshMe()
      }
    },

    /** 登录收尾：写 token、拉起邀请绑定 */
    applyLoginResult(res) {
      setToken(res.token)
      this.token = res.token
      this.user = res.user
      this.isCoach = !!res.isCoach
      this.coachId = res.coachId || 0
      tryBindInvite()
    },

    /** 小程序静默登录（wx.login code 换 token） */
    async loginByWechat(customCode) {
      let code = customCode
      if (!code) {
        const res = await uni.login({ provider: 'weixin' })
        code = res.code
      }
      const data = await request({ url: '/api/v1/auth/login', method: 'POST', data: { code } })
      this.applyLoginResult(data)
      return data
    },

    /** 需要登录态时调用：MP 静默登录；H5 无 token 则跳登录页并 reject */
    async ensureLogin() {
      if (this.token) return
      // #ifdef MP-WEIXIN
      await this.loginByWechat()
      return
      // #endif
      // #ifdef H5
      const pages = getCurrentPages()
      const cur = pages.length ? pages[pages.length - 1] : null
      const route = cur ? cur.route : ''
      if (route === 'pages/login/login') throw new Error('unauthenticated')
      // 携带来源页与参数，登录成功后跳回
      const opts = (cur && cur.$page && cur.$page.options) || {}
      const qs = Object.entries(opts).map(([k, v]) => `${k}=${encodeURIComponent(v)}`).join('&')
      const redirect = '/' + route + (qs ? '?' + qs : '')
      uni.reLaunch({ url: '/pages/login/login?redirect=' + encodeURIComponent(redirect) })
      throw new Error('unauthenticated')
      // #endif
    },

    /** 已有 token 时刷新用户信息（token 失效由 request 层兜底清理） */
    async refreshMe() {
      try {
        const res = await request({ url: '/api/v1/me', silent: true })
        this.user = res.user
        this.isCoach = !!res.isCoach
        this.coachId = res.coachId || 0
      } catch (e) {
        // token 失效：保持清理后的状态，等待下次操作重新登录
      }
    },

    logout() {
      request({ url: '/api/v1/auth/logout', method: 'POST', silent: true }).catch(() => {})
      setToken('')
      this.token = ''
      this.user = null
      this.isCoach = false
      this.coachId = 0
      uni.showToast({ title: '已退出登录', icon: 'none' })
    }
  }
})
