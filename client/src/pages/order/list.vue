<template>
  <view class="md-page">
    <M3TopAppBar title="我的订单" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 状态 tab -->
      <M3Tabs :tabs="TABS" :model-value="tab" @update:model-value="onTab" class="order-tabs" />

      <!-- 骨架屏 -->
      <template v-if="loading && !list.length">
        <M3Card v-for="i in 3" :key="i" variant="filled" class="sk-card">
          <view class="sk-inner">
            <view class="sk-row">
              <M3Skeleton variant="text" width="45%" height="14" />
              <M3Skeleton variant="text" width="48px" height="14" />
            </view>
            <M3Skeleton variant="text" width="65%" height="16" />
            <view class="sk-row">
              <M3Skeleton variant="text" width="80px" height="20" />
              <M3Skeleton variant="rect" width="96px" height="32" />
            </view>
          </view>
        </M3Card>
      </template>

      <!-- 空态 -->
      <M3EmptyState
        v-else-if="!list.length"
        icon="receipt_long"
        :title="tab === 'all' ? '还没有订单' : '该状态下暂无订单'"
        :description="tab === 'all' ? '去挑个课包，开启训练计划吧' : ''"
      >
        <M3Button v-if="tab === 'all'" variant="filled" icon="shopping_bag" @click="goCourse">去逛逛</M3Button>
      </M3EmptyState>

      <!-- 订单列表 -->
      <template v-else>
        <M3Card v-for="o in list" :key="o.id" variant="filled" class="od-card">
          <view class="od-inner">
            <view class="od-head">
              <text class="od-no md-ellipsis">{{ o.orderNo }}</text>
              <view class="st" :style="{ color: o.statusColor }">
                <view class="st-dot" :style="{ background: o.statusColor }" />
                <text class="st-label">{{ o.statusText }}</text>
              </view>
            </view>
            <view class="od-body">
              <text class="od-title md-ellipsis">{{ o.title }}</text>
              <text class="od-time">{{ o.timeText }}</text>
            </view>
            <view class="od-tags" v-if="o.newcomerUsed || o.promoUsed || o.groupBuyId > 0">
              <text v-if="o.newcomerUsed" class="od-tag">新人价</text>
              <text v-if="o.promoUsed" class="od-tag">活动价</text>
              <text v-if="o.groupBuyId > 0" class="od-tag">拼团</text>
            </view>
            <view class="od-foot">
              <view class="od-amount-wrap">
                <text class="od-amount">¥{{ o.amountYuan }}</text>
                <text v-if="o.couponAmount > 0" class="od-cut">已优惠¥{{ o.couponCutYuan }}</text>
              </view>
              <view class="od-actions">
                <M3Button
                  v-if="o.status === 'pending'"
                  size="sm"
                  variant="filled"
                  icon="payments"
                  :loading="payingId === o.id"
                  @click="onPay(o)"
                >去支付</M3Button>
                <M3Button
                  v-if="o.status === 'paid'"
                  size="sm"
                  variant="outlined"
                  @click="askRefund(o)"
                >申请退款</M3Button>
              </view>
            </view>
          </view>
        </M3Card>

        <!-- 加载/到底提示 -->
        <view v-if="loading && list.length" class="od-more">
          <M3Progress type="linear" indeterminate />
        </view>
        <text v-else-if="finished && list.length" class="od-more-text">没有更多了</text>
        <view v-else-if="!finished && list.length" class="od-more-space" />
      </template>
    </view>

    <!-- 退款申请 -->
    <M3Dialog :open="!!refundOrder" title="申请退款" icon="help" :dismissible="false">
      <text class="refund-tip">提交后由工作室审核，未开卡期限内可全额退。</text>
      <M3TextField
        v-model="refundReason"
        label="退款原因（选填）"
        placeholder="选填"
        :maxlength="100"
        label-bg="var(--md-surface-container-high)"
        class="refund-input"
      />
      <template #actions>
        <M3Button variant="text" @click="refundOrder = null">取消</M3Button>
        <M3Button variant="filled" :loading="refunding" @click="doRefund">提交申请</M3Button>
      </template>
    </M3Dialog>

    <!-- H5 扫码支付弹层 -->
    <M3PaySheet :open="payOpen" :qr="payQr" :amount="payAmount" @close="payOpen = false" />
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onShow, onReachBottom, onPageScroll } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { payOrder } from '@/utils/pay'
import { useAuthStore } from '@/stores/auth'
import { yuan, fmtTime } from '@/utils/format'

const auth = useAuthStore()

// 状态 → 中文 + MD3 语义色
const STATUS = {
  pending: ['待支付', 'var(--md-tertiary)'],
  paid: ['已支付', 'var(--md-primary)'],
  closed: ['已关闭', 'var(--md-outline)'],
  refunding: ['退款中', 'var(--md-error)'],
  refunded: ['已退款', 'var(--md-on-surface-variant)']
}

// 状态 tab（后端 GET /orders?status= 逗号分隔多状态，服务端过滤）
const TABS = [
  { value: 'all', label: '全部' },
  { value: 'pending', label: '待支付' },
  { value: 'paid', label: '已支付' },
  { value: 'refund', label: '退款' }
]
const TAB_STATUS = {
  all: '',
  pending: 'pending',
  paid: 'paid',
  refund: 'refunding,refunded'
}

const list = ref([])
const page = ref(1)
const total = ref(0)
const loading = ref(false)
const finished = ref(false)
const tab = ref('all')
const scrolled = ref(false)

const payingId = ref(0)
const payOpen = ref(false)
const payQr = ref('')
const payAmount = ref('')

const refundOrder = ref(null)
const refundReason = ref('')
const refunding = ref(false)

onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

// 切 tab：服务端按状态重查
function onTab(v) {
  if (v === tab.value) return
  tab.value = v
  reload()
}

onShow(() => {
  page.value = 1
  finished.value = false
  load()
})

onReachBottom(() => {
  if (!finished.value && !loading.value) load()
})

function decorate(o) {
  const [statusText, statusColor] = STATUS[o.status] || [o.status, 'var(--md-on-surface-variant)']
  return {
    ...o,
    statusText,
    statusColor,
    amountYuan: yuan(o.amount),
    couponCutYuan: yuan(o.couponAmount || 0),
    timeText: fmtTime(o.createdAt)
  }
}

async function load() {
  try {
    await auth.ensureLogin()
  } catch (e) { return } // H5 未登录已跳登录页
  loading.value = true
  try {
    const st = TAB_STATUS[tab.value]
    const url = `/api/v1/orders?page=${page.value}&page_size=10` + (st ? `&status=${st}` : '')
    const data = await request({ url })
    const rows = (data && data.list) || []
    const merged = page.value === 1 ? rows.map(decorate) : list.value.concat(rows.map(decorate))
    list.value = merged
    total.value = (data && data.total) || 0
    page.value += 1
    finished.value = merged.length >= total.value
  } catch (e) { /* 错误 toast 已由请求层处理 */ } finally {
    loading.value = false
  }
}

function reload() {
  page.value = 1
  finished.value = false
  load()
}

function goCourse() {
  uni.reLaunch({ url: '/pages/course/list' })
}

// 重新支付：统一 payOrder（MP 拉起收银台 / H5 弹二维码），禁止自行调 prepay
async function onPay(o) {
  if (payingId.value) return
  payingId.value = o.id
  payAmount.value = yuan(o.amount)
  try {
    const res = await payOrder(o.id, {
      onQrCode: (dataUrl) => {
        payQr.value = dataUrl
        payOpen.value = true
      }
    })
    if (res.paid) {
      payOpen.value = false
      uni.showToast({ title: '支付成功', icon: 'success' })
      reload()
    } else if (res.timeout) {
      uni.showToast({ title: '支付超时，可稍后重试', icon: 'none' })
    }
  } catch (e) { /* 用户取消支付 / 请求层错误 */ } finally {
    payingId.value = 0
  }
}

function askRefund(o) {
  refundReason.value = ''
  refundOrder.value = o
}

async function doRefund() {
  const o = refundOrder.value
  if (!o || refunding.value) return
  refunding.value = true
  try {
    await request({
      url: `/api/v1/orders/${o.id}/refund`,
      method: 'POST',
      data: { reason: refundReason.value.trim() }
    })
    refundOrder.value = null
    uni.showToast({ title: '已提交审核', icon: 'success' })
    reload()
  } catch (e) { /* toast 已处理 */ } finally {
    refunding.value = false
  }
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 0 16px 32px;
}

.order-tabs {
  margin: 0 -16px 12px;
}

.sk-card {
  margin-bottom: 12px;
}
.sk-inner {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.sk-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.od-card {
  margin-bottom: 12px;
}
.od-inner {
  padding: 14px 16px;
}
.od-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.od-no {
  flex: 1;
  min-width: 0;
  color: var(--md-on-surface-variant);
  @include md-body-small;
}
.od-body {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.od-title {
  color: var(--md-on-surface);
  @include md-title-medium;
}
.od-time {
  color: var(--md-on-surface-variant);
  @include md-body-small;
}
.od-tags {
  margin-top: 8px;
  display: flex;
  gap: 6px;
}
.od-tag {
  padding: 2px 8px;
  border-radius: $md-shape-full;
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);
  @include md-label-small;
}
.od-foot {
  margin-top: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.od-amount-wrap {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}
.od-amount {
  color: var(--md-primary);
  font-weight: 600;
  @include md-title-medium;
}
.od-cut {
  color: var(--md-tertiary);
  @include md-body-small;
}
.od-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.od-more {
  padding: 8px 0;
}
.od-more-text {
  display: block;
  text-align: center;
  padding: 12px 0;
  color: var(--md-on-surface-variant);
  @include md-body-small;
}
.od-more-space {
  height: 8px;
}

.refund-tip {
  display: block;
  line-height: 20px;
}
.refund-input {
  margin-top: 12px;
  display: block;
}

// 状态：颜色圆点 + label-small
.st {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  &-dot {
    width: 8px;
    height: 8px;
    border-radius: $md-shape-full;
  }
  &-label {
    @include md-label-small;
  }
}
</style>
