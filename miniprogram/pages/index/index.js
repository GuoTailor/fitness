const app = getApp()
const { request } = require('../../utils/request')

Page({
  data: {
    user: null,
    banners: [],
    hotPackages: [],
    coaches: [],
    devCode: '',
  },

  onShow() {
    this.setData({ user: app.globalData.user })
    this.loadHome()
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

  onDevInput(e) {
    this.setData({ devCode: e.detail.value })
  },
  async onDevLogin() {
    if (!this.data.devCode) return
    await app.loginAs(this.data.devCode)
    this.setData({ user: app.globalData.user })
    wx.showToast({ title: '已切换身份', icon: 'none' })
  },
})
