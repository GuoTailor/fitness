// 教练入驻申请：表单 + 资质图上传（1~6 张）+ 我的申请状态
const app = getApp()
const { request, uploadFile } = require('../../utils/request')

const STATUS_TEXT = { 0: '待审核', 1: '已通过', 2: '已驳回' }

Page({
  data: {
    form: { name: '', gender: 0, years: '', specialties: '', intro: '', phone: '' },
    certImages: [],       // 站内上传 URL
    uploading: false,
    submitting: false,
    mine: null,           // 已有申请（状态展示）
  },

  onShow() {
    this.loadMine()
  },

  async loadMine() {
    await app.ensureLogin().catch(() => {})
    if (!wx.getStorageSync('token')) return
    try {
      const data = await request({ url: '/api/v1/coach-applications/mine' })
      if (data.exists) {
        const rec = data.application
        rec.statusText = STATUS_TEXT[rec.status] || ''
        rec.createdText = String(rec.createdAt).replace('T', ' ').slice(0, 16)
      }
      this.setData({ mine: data.exists ? data.application : null })
    } catch (e) { /* 未登录容忍 */ }
  },

  onInput(e) {
    const key = e.currentTarget.dataset.key
    this.setData({ [`form.${key}`]: e.detail.value })
  },

  onGender(e) {
    this.setData({ 'form.gender': Number(e.currentTarget.dataset.v) })
  },

  // 资质图：chooseMedia → 逐张上传
  async onAddCert() {
    const left = 6 - this.data.certImages.length
    if (left <= 0) return wx.showToast({ title: '最多 6 张', icon: 'none' })
    const res = await wx.chooseMedia({
      count: left, mediaType: ['image'], sizeType: ['compressed'],
    }).catch(() => null)
    if (!res || !res.tempFiles || !res.tempFiles.length) return
    this.setData({ uploading: true })
    try {
      const urls = []
      for (const f of res.tempFiles) {
        const url = await uploadFile(f.tempFilePath)
        urls.push(url)
      }
      this.setData({ certImages: this.data.certImages.concat(urls) })
    } catch (e) { /* toast 已处理 */ } finally {
      this.setData({ uploading: false })
    }
  },

  onRemoveCert(e) {
    const i = e.currentTarget.dataset.index
    const arr = this.data.certImages.slice()
    arr.splice(i, 1)
    this.setData({ certImages: arr })
  },

  onPreview(e) {
    wx.previewImage({ urls: this.data.certImages, current: e.currentTarget.dataset.url })
  },

  async onSubmit() {
    const f = this.data.form
    const name = (f.name || '').trim()
    if (name.length < 2) return wx.showToast({ title: '请填写真实姓名', icon: 'none' })
    if (!this.data.certImages.length) return wx.showToast({ title: '请上传资质证书', icon: 'none' })
    this.setData({ submitting: true })
    try {
      await request({
        url: '/api/v1/coach-applications', method: 'POST',
        data: {
          name, gender: f.gender, years: parseInt(f.years, 10) || 0,
          specialties: f.specialties, intro: f.intro, phone: f.phone,
          certImages: JSON.stringify(this.data.certImages),
        },
      })
      wx.showToast({ title: '已提交，等待审核', icon: 'success' })
      this.loadMine()
    } catch (e) { /* toast 已处理 */ } finally {
      this.setData({ submitting: false })
    }
  },
})
