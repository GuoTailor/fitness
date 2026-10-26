const app = getApp()
const { request } = require('../../utils/request')
const referral = require('../../utils/referral')

Page({
  data: {
    user: null,
    banners: [],
    hotPackages: [],
    coaches: [],
  },

  onLoad(options) {
    // 老带新：从分享卡片落地，暂存邀请码待登录后绑定
    if (options && options.inviter) referral.captureInvite(options.inviter)
  },

  onShow() {
    this.setData({ user: app.globalData.user })
    this.loadHome()
    referral.tryBindInvite()
  },

  // 老带新：转发卡片带上邀请码
  onShareAppMessage() {
    return referral.shareCard('一起来健身，约课更方便', '/pages/index/index')
  },

  async loadHome() {
    try {
      const data = await request({ url: '/api/v1/home' })
      this.setData({ banners: data.banners || [], hotPackages: data.hotPackages || [], coaches: data.coaches || [] })
    } catch (e) { /* 弱网容忍 */ }
  },

  onBannerTap(e) {
    const { type, value } = e.currentTarget.dataset
    if (type === 'package') wx.navigateTo({ url: '/pages/course/detail?id=' + value })
    else if (type === 'coach') wx.navigateTo({ url: '/pages/coach/detail?id=' + value })
    else if (type === 'groupbuy') wx.navigateTo({ url: '/pages/groupbuy/list?activityId=' + value })
  },

  onPkgTap(e) {
    wx.navigateTo({ url: '/pages/course/detail?id=' + e.currentTarget.dataset.id })
  },
  onCoachTap(e) {
    wx.navigateTo({ url: '/pages/coach/detail?id=' + e.currentTarget.dataset.id })
  },
  goCourses() {
    wx.switchTab({ url: '/pages/course/list' })
  },
  goCoaches() {
    wx.navigateTo({ url: '/pages/coach/list' })
  },
  goCalendar() {
    wx.navigateTo({ url: '/pages/booking/calendar' })
  },

  async onLogin() {
    await app.ensureLogin()
    this.setData({ user: app.globalData.user })
  },
})
