const app = getApp()
const { request } = require('../../utils/request')

Page({
  data: {
    id: 0,
    detail: null,
    selectedSku: -1,
    coupons: [],       // 当前 SKU 可用券（后端已按门槛/范围过滤）
    couponIndex: -1,   // -1=不使用优惠券
    payYuan: '',       // 预估合计（新人价/活动价与券取低，最终以下单服务端为准）
    buying: false,
    reviewTab: 'reviews',
    // 问答
    qaList: [], qaTotal: 0, askOpen: false, askContent: '', askAnonymous: false, asking: false,
  },

  onLoad(query) {
    this.setData({ id: query.id })
    this.load()
  },

  async load() {
    const data = await request({ url: `/api/v1/packages/${this.data.id}` })
    // 价格分转元展示；新人价仅对可享用户展示、活动价仅活动期内展示（均服务端判定）
    data.skus.forEach((s) => {
      s.priceYuan = (s.price / 100).toFixed(0)
      const promo = s.promoActive && s.promoPrice ? s.promoPrice : 0
      const nc = s.newcomerEligible && s.newcomerPrice ? s.newcomerPrice : 0
      if (nc && (!promo || nc < promo)) {
        s.newcomerYuan = (nc / 100).toFixed(0)
      }
      // 主展示价：新人价更低用新人价，否则活动价（同额展示活动价，与结算规则一致）
      if (s.newcomerYuan) {
        s.dispYuan = s.newcomerYuan
        s.dispTag = '新人价'
      } else if (promo) {
        s.dispYuan = (promo / 100).toFixed(0)
        s.dispTag = '限时价'
      }
    })
    this.setData({ detail: data, selectedSku: data.skus.length === 1 ? 0 : -1 })
    if (data.skus.length === 1) this.loadCoupons(data.skus[0].id)
    this.startCountdown()
  },

  onShow() { this.startCountdown() },
  onHide() { this.stopCountdown() },
  onUnload() { this.stopCountdown() },

  // 活动价倒计时（选中规格活动生效时展示）
  startCountdown() {
    this.stopCountdown()
    const tick = () => {
      const { detail, selectedSku } = this.data
      const sku = detail && detail.skus && selectedSku >= 0 ? detail.skus[selectedSku] : null
      if (!sku || !sku.promoActive || !sku.promoEnd) {
        if (this.data.promoLeft) this.setData({ promoLeft: '' })
        return
      }
      const left = new Date(sku.promoEnd).getTime() - Date.now()
      if (left <= 0) {
        this.setData({ promoLeft: '' })
        return
      }
      const d = Math.floor(left / 86400000)
      const h = String(Math.floor((left % 86400000) / 3600000)).padStart(2, '0')
      const m = String(Math.floor((left % 3600000) / 60000)).padStart(2, '0')
      const sec = String(Math.floor((left % 60000) / 1000)).padStart(2, '0')
      this.setData({ promoLeft: (d > 0 ? d + '天' : '') + h + ':' + m + ':' + sec })
    }
    tick()
    this._promoTimer = setInterval(tick, 1000)
  },
  stopCountdown() {
    if (this._promoTimer) {
      clearInterval(this._promoTimer)
      this._promoTimer = null
    }
  },

  onSku(e) {
    this.setData({ selectedSku: Number(e.currentTarget.dataset.index) })
    const sku = this.data.detail.skus[this.data.selectedSku]
    this.loadCoupons(sku.id)
    this.startCountdown()
  },
  onTab(e) {
    this.setData({ reviewTab: e.currentTarget.dataset.tab })
    if (e.currentTarget.dataset.tab === 'qa' && !this.data.qaList.length) this.loadQa()
  },

  // ---- 课程问答 ----
  async loadQa() {
    try {
      const data = await request({ url: `/api/v1/packages/${this.data.id}/qa?page=1&pageSize=20` })
      const list = (data.list || []).map((q) => {
        q.timeText = String(q.createdAt || '').replace('T', ' ').slice(0, 16)
        ;(q.answers || []).forEach((a) => {
          a.timeText = String(a.createdAt || '').replace('T', ' ').slice(0, 16)
        })
        return q
      })
      this.setData({ qaList: list, qaTotal: data.total || 0 })
    } catch (e) { /* 静默：问答加载失败不阻塞详情 */ }
  },
  onToggleAsk() { this.setData({ askOpen: !this.data.askOpen }) },
  onAskInput(e) { this.setData({ askContent: e.detail.value }) },
  onAskAnon(e) { this.setData({ askAnonymous: e.detail.value }) },
  async onSubmitAsk() {
    const app2 = app
    await app2.ensureLogin()
    if (this.data.askContent.trim().length < 5) {
      return wx.showToast({ title: '问题至少 5 个字', icon: 'none' })
    }
    this.setData({ asking: true })
    try {
      await request({
        url: '/api/v1/qa/questions', method: 'POST',
        data: { packageId: Number(this.data.id), content: this.data.askContent.trim(), anonymous: this.data.askAnonymous },
      })
      wx.showToast({ title: '已提交', icon: 'success' })
      this.setData({ askOpen: false, askContent: '', askAnonymous: false })
      this.loadQa()
    } catch (e) { /* toast 已处理 */ } finally { this.setData({ asking: false }) }
  },
  onCoach(e) {
    wx.navigateTo({ url: '/pages/coach/detail?id=' + e.currentTarget.dataset.id })
  },

  async loadCoupons(skuId) {
    try {
      const data = await request({ url: `/api/v1/coupons/usable?skuId=${skuId}` })
      const items = (data.items || []).map((c) => ({ ...c, benefitYuan: (c.benefit / 100).toFixed(2) }))
      this.setData({ coupons: items, couponIndex: items.length ? 0 : -1 })
    } catch (e) {
      this.setData({ coupons: [], couponIndex: -1 })
    }
    this.computePay()
  },

  async onPickCoupon() {
    const { coupons } = this.data
    if (!coupons.length) {
      return wx.showToast({ title: '该课包暂无可用券，可去领券中心看看', icon: 'none' })
    }
    const items = ['不使用优惠券'].concat(
      coupons.map((c) => `${c.name} ${c.label}${c.giftSessions ? '' : `（省¥${c.benefitYuan}）`}`))
    wx.showActionSheet({
      itemList: items,
      success: (r) => {
        this.setData({ couponIndex: r.tapIndex - 1 })
        this.computePay()
      },
    })
  },
  onGoCouponCenter() {
    wx.navigateTo({ url: '/pages/coupon/center' })
  },

  computePay() {
    const { detail, selectedSku, couponIndex, coupons } = this.data
    if (!detail || selectedSku < 0) {
      this.setData({ payYuan: '' })
      return
    }
    const sku = detail.skus[selectedSku]
    let pay = sku.price
    const c = couponIndex >= 0 ? coupons[couponIndex] : null
    if (c && !c.giftSessions) pay = c.finalPay // 赠课券不降价
    if (sku.newcomerEligible && sku.newcomerPrice && sku.newcomerPrice < pay) pay = sku.newcomerPrice
    if (sku.promoActive && sku.promoPrice && sku.promoPrice < pay) pay = sku.promoPrice
    this.setData({ payYuan: (pay / 100).toFixed(2) })
  },

  async onBuy() {
    const app2 = app
    await app2.ensureLogin()
    const { detail, selectedSku, coupons, couponIndex } = this.data
    if (selectedSku < 0) return wx.showToast({ title: '请选择规格', icon: 'none' })
    const sku = detail.skus[selectedSku]
    const couponId = couponIndex >= 0 && coupons[couponIndex] ? coupons[couponIndex].couponId : 0
    this.setData({ buying: true })
    try {
      const order = await request({
        url: '/api/v1/orders', method: 'POST', data: { skuId: sku.id, couponId },
      })
      // 支付：mock 通道直接成功；真实通道返回 payParams 拉起 wx.requestPayment
      const payRes = await request({
        url: `/api/v1/orders/${order.id}/prepay`, method: 'POST', data: {},
      })
      if (payRes.mockPaid) {
        wx.showModal({
          title: '支付成功',
          content: `已购买「${order.title}」，实付 ¥${(order.amount / 100).toFixed(2)}`,
          confirmText: '去预约',
          cancelText: '看课包',
          success: (r) => {
            if (r.confirm) wx.navigateTo({ url: '/pages/training/index' })
          },
        })
      } else if (payRes.payParams) {
        const p = payRes.payParams
        await new Promise((resolve, reject) => {
          wx.requestPayment({
            timeStamp: p.timeStamp, nonceStr: p.nonceStr, package: p.package,
            signType: p.signType, paySign: p.paySign,
            success: resolve, fail: reject,
          })
        })
        wx.showModal({
          title: '支付成功',
          content: `已购买「${order.title}」，课时将在支付回调后入账`,
          showCancel: false,
        })
      } else {
        wx.showToast({ title: '支付通道未就绪', icon: 'none' })
      }
    } catch (e) {
      // request 封装已 toast
    } finally {
      this.setData({ buying: false })
    }
  },
})
