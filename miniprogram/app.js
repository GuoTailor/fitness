// app.js 全局登录态与请求配置
const { login } = require('./utils/auth')

App({
  globalData: {
    // 后端地址：开发者工具本地联调指向本机；上线改为正式域名（需备案+小程序后台配置）
    apiBase: 'http://127.0.0.1:8080',
    user: null,
    isCoach: false,
    coachId: 0,
  },

  onLaunch() {
    // 已有 token 时静默刷新用户信息；没有则等待页面触发登录
    if (wx.getStorageSync('token')) {
      this.refreshMe()
    }
  },

  /** wx.login → 后端换 token（后端 mock 模式下 code 即身份） */
  async ensureLogin() {
    if (wx.getStorageSync('token')) return this.globalData
    return login()
  },

  /** 用指定身份登录（开发用：mock-user-1 / mock-coach-1 等固定账号） */
  async loginAs(code) {
    return login(code)
  },

  async refreshMe() {
    const { request } = require('./utils/request')
    try {
      const res = await request({ url: '/api/v1/me', method: 'GET' })
      this.globalData.user = res.user
      this.globalData.isCoach = res.isCoach
      this.globalData.coachId = res.coachId
    } catch (e) {
      // token 失效：清掉本地态，等用户下次操作重新登录
      if (e.code === 40101) {
        wx.removeStorageSync('token')
        this.globalData.user = null
      }
    }
  },
})
