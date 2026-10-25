const app = getApp()
const { request, uploadFile } = require('../../utils/request')

Page({
  data: {
    user: null,
    isCoach: false,
    phoneBinding: false,
    // 资料编辑
    editing: false,
    editNickname: '',
    editAvatar: '',
    savingProfile: false,
  },

  onShow() {
    this.setData({ user: app.globalData.user, isCoach: app.globalData.isCoach })
    if (app.globalData.user) app.refreshMe()
  },

  async onLogin() {
    await app.ensureLogin()
    this.setData({ user: app.globalData.user, isCoach: app.globalData.isCoach })
  },

  // ---- 资料编辑（头像 chooseAvatar → 站内上传；昵称 type=nickname 微信昵称快速填充）----
  onEditProfile() {
    this.setData({ editing: true, editNickname: (this.data.user && this.data.user.nickname) || '', editAvatar: '' })
  },
  onCancelEdit() { this.setData({ editing: false }) },
  onEditNickname(e) { this.setData({ editNickname: e.detail.value }) },

  async onChooseAvatar(e) {
    const path = e.detail.avatarUrl
    if (!path) return
    wx.showLoading({ title: '上传中…' })
    try {
      const url = await uploadFile(path)
      this.setData({ editAvatar: url })
    } catch (err) { /* toast 已处理 */ } finally { wx.hideLoading() }
  },

  async onSaveProfile() {
    const nickname = (this.data.editNickname || '').trim()
    if (!nickname) return wx.showToast({ title: '请填写昵称', icon: 'none' })
    this.setData({ savingProfile: true })
    try {
      await request({
        url: '/api/v1/me', method: 'PUT',
        data: { nickname, avatarUrl: this.data.editAvatar || '' },
      })
      await app.refreshMe()
      this.setData({ user: app.globalData.user, editing: false })
      wx.showToast({ title: '已保存', icon: 'success' })
    } catch (e) { /* toast 已处理 */ } finally { this.setData({ savingProfile: false }) }
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
