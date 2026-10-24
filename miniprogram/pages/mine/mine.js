const app = getApp()
const { request } = require('../../utils/request')

Page({
  data: {
    user: null,
    isCoach: false,
    phoneBinding: false,
  },

  onShow() {
    this.setData({ user: app.globalData.user, isCoach: app.globalData.isCoach })
    if (app.globalData.user) app.refreshMe()
  },

  async onLogin() {
    await app.ensureLogin()
    this.setData({ user: app.globalData.user, isCoach: app.globalData.isCoach })
  },

  /** 手机号快速授权：open-type=button 拿 phone code → 后端解出手机号 */
  async onGetPhone(e) {
    if (!e.detail.code) return
    this.setData({ phoneBinding: true })
    try {
      await request({
        url: '/api/v1/auth/phone',
        method: 'POST',
        data: { code: e.detail.code },
      })
      await app.refreshMe()
      this.setData({ user: app.globalData.user })
      wx.showToast({ title: '绑定成功', icon: 'success' })
    } finally {
      this.setData({ phoneBinding: false })
    }
  },

  onLogout() {
    wx.removeStorageSync('token')
    app.globalData.user = null
    app.globalData.isCoach = false
    this.setData({ user: null, isCoach: false })
  },
})
