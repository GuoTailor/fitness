const { request } = require('../../utils/request')

Page({
  data: { manualCode: '', lastResult: null, lastError: '' },

  async onScan() {
    try {
      const scan = await wx.scanCode({ onlyFromCamera: false })
      await this.doVerify(scan.result)
    } catch (e) { /* 用户取消 */ }
  },

  async onManualInput(e) { this.setData({ manualCode: e.detail.value }) },

  async onManualSubmit() {
    if (!this.data.manualCode) return
    await this.doVerify(this.data.manualCode.trim())
  },

  async doVerify(code) {
    try {
      const r = await request({ url: '/api/v1/coach/verify', method: 'POST', data: { code } })
      this.setData({
        lastResult: r, lastError: '',
        manualCode: r.alreadyVerified ? this.data.manualCode : '',
      })
      wx.vibrateShort()
    } catch (e) {
      this.setData({ lastResult: null, lastError: '核销失败：' + (e.message || '码无效') })
    }
  },
})
