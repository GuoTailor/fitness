<template>
  <view class="md-page">
    <M3TopAppBar title="拼团" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 首屏骨架 -->
      <template v-if="firstLoading">
        <M3Card variant="filled" class="gb-skeleton">
          <view class="gb-skeleton-inner">
            <M3Skeleton variant="text" width="60%" height="16" />
            <M3Skeleton variant="text" width="40%" height="14" />
            <M3Skeleton variant="rect" width="100%" height="36" />
          </view>
        </M3Card>
        <M3Card v-for="i in 2" :key="i" variant="filled" class="gb-skeleton">
          <view class="gb-skeleton-inner">
            <M3Skeleton variant="text" width="45%" height="14" />
            <M3Skeleton variant="rect" width="100%" height="4" />
          </view>
        </M3Card>
      </template>

      <template v-else>
        <!-- 课包头卡：拼团价 + 开新团 -->
        <M3Card v-if="sku" variant="filled" class="gb-head">
          <view class="gb-head-inner">
            <view v-if="actTitle" class="gb-act-badge">
              <M3Icon name="campaign" :size="14" />
              <text>限时拼团 · {{ actTitle }}</text>
            </view>
            <view class="gb-title md-ellipsis-2">{{ title }}</view>
            <view class="gb-price-row">
              <view class="gb-price">
                <text class="gb-price-symbol">¥</text>
                <text class="gb-price-num">{{ yuan(sku.groupPrice) }}</text>
                <text class="gb-price-label">拼团价</text>
              </view>
              <text class="gb-origin">单买 ¥{{ yuan(sku.price) }}</text>
            </view>
            <text class="gb-cap">{{ sku.groupCapacity }}人成团 · {{ sku.sessions }}课时</text>
            <M3Button
              variant="filled"
              icon="group_add"
              block
              class="gb-open-btn"
              :loading="opening"
              :disabled="!skuId"
              @click="onOpen"
            >发起新团</M3Button>
          </view>
        </M3Card>

        <!-- 进行中的团 -->
        <view v-if="groups.length" class="gb-sec">
          <text class="gb-sec-title">进行中的团（{{ groups.length }}）</text>
        </view>
        <M3Card
          v-for="g in groups"
          :key="g.id"
          variant="outlined"
          clickable
          class="gb-card"
          @click="goDetail(g.id)"
        >
          <view class="gb-card-inner">
            <view class="gb-main">
              <view class="gb-leader-row">
                <M3Avatar :name="g.leaderName" :size="36" />
                <view class="gb-leader-info">
                  <text class="gb-leader md-ellipsis">{{ g.leaderName }} 的团</text>
                  <text class="gb-end">{{ g.endText }} 截止</text>
                </view>
              </view>
              <view class="gb-progress">
                <M3Progress type="linear" :model-value="progressOf(g)" />
                <view class="gb-progress-meta">
                  <text class="gb-count">{{ g.paidCount }}/{{ g.capacity }} 人已上车</text>
                  <text class="gb-lack">还差 {{ Math.max(0, g.capacity - g.paidCount) }} 人</text>
                </view>
              </view>
            </view>
            <view class="gb-side">
              <text class="gb-side-price">¥{{ g.priceYuan }}</text>
              <text class="gb-side-join">参团 ›</text>
            </view>
          </view>
        </M3Card>

        <!-- 空态 -->
        <M3EmptyState
          v-if="!groups.length"
          icon="group_add"
          title="还没有人开团"
          :description="sku ? '来做第一个，拉上好友一起练' : ''"
        >
          <M3Button v-if="sku" variant="filled" icon="group_add" :loading="opening" @click="onOpen">立即开团</M3Button>
        </M3EmptyState>
      </template>
    </view>

    <!-- H5 扫码支付弹层 -->
    <M3PaySheet :open="payOpen" :qr="payQr" :amount="payAmount" @close="payOpen = false" />
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onPageScroll } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { payOrder } from '@/utils/pay'
import { useAuthStore } from '@/stores/auth'
import { yuan, fmtTime } from '@/utils/format'

const auth = useAuthStore()
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

// 两种落地方式：pkgId+skuId（课程详情进入）、activityId（首页 Banner 落地）
const pkgId = ref(0)
const skuId = ref(0)
const actId = ref(0)
const actTitle = ref('')
const title = ref('')
const sku = ref(null)

const groups = ref([])
const firstLoading = ref(true)
const opening = ref(false)

const payOpen = ref(false)
const payQr = ref('')
const payAmount = ref('')

onLoad(async (query) => {
  await auth.ensureLogin().catch(() => {})
  pkgId.value = parseInt((query && query.pkgId) || 0, 10) || 0
  skuId.value = parseInt((query && query.skuId) || 0, 10) || 0
  actId.value = parseInt((query && query.activityId) || 0, 10) || 0
  await load()
  firstLoading.value = false
})

async function load() {
  // Banner 落地：先换出 pkgId/skuId
  if (actId.value && !pkgId.value) {
    const act = await request({ url: `/api/v1/group-buy-activities/${actId.value}` }).catch(() => null)
    if (act) {
      pkgId.value = act.packageId
      skuId.value = act.skuId
      actTitle.value = act.title || ''
    }
  }
  if (pkgId.value) {
    const detail = await request({ url: `/api/v1/packages/${pkgId.value}` }).catch(() => null)
    if (detail) {
      const s = (detail.skus || []).find((x) => x.id === skuId.value)
      if (s) {
        title.value = detail.title || ''
        sku.value = s
      }
    }
  }
  if (!skuId.value) return
  try {
    const data = await request({ url: `/api/v1/group-buys?skuId=${skuId.value}` })
    groups.value = ((data && data.list) || []).map((g) => ({
      ...g,
      priceYuan: yuan(g.price),
      endText: fmtTime(g.expireAt).slice(5, 16)
    }))
  } catch (e) {
    // 错误已由请求层 toast
  }
}

function progressOf(g) {
  if (!g.capacity) return 0
  return Math.round((g.paidCount * 100) / g.capacity)
}

// 开新团：返回待支付订单 → 统一 payOrder 支付
async function onOpen() {
  if (opening.value || !skuId.value) return
  opening.value = true
  try {
    const order = await request({ url: '/api/v1/group-buys', method: 'POST', data: { skuId: skuId.value } })
    payAmount.value = yuan(order.amount || (sku.value && sku.value.groupPrice) || 0)
    const payRes = await payOrder(order.id, {
      onQrCode: (dataUrl) => {
        payQr.value = dataUrl
        payOpen.value = true
      }
    })
    if (payRes.paid) {
      payOpen.value = false
      uni.showToast({ title: '开团成功，快邀请好友', icon: 'none' })
      uni.redirectTo({ url: `/pages/groupbuy/detail?gbid=${order.groupBuyId}` })
    }
  } catch (e) {
    // 用户取消支付 / 请求层错误（toast 已处理）
  } finally {
    opening.value = false
  }
}

function goDetail(id) {
  uni.navigateTo({ url: `/pages/groupbuy/detail?gbid=${id}` })
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

.gb-skeleton {
  margin-bottom: 12px;
}
.gb-skeleton-inner {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

// 头卡
.gb-head {
  margin-bottom: 16px;
}
.gb-head-inner {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.gb-act-badge {
  align-self: flex-start;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 10px;
  border-radius: $md-shape-full;
  background: var(--md-error-container);
  color: var(--md-on-error-container);
  @include md-label-medium;
}
.gb-title {
  color: var(--md-on-surface);
  @include md-title-large;
  line-height: 26px;
}
.gb-price-row {
  display: flex;
  align-items: baseline;
  gap: 10px;
}
.gb-price {
  display: flex;
  align-items: baseline;
  gap: 2px;
  color: var(--md-primary);
  &-symbol {
    @include md-title-medium;
    font-weight: 700;
  }
  &-num {
    @include md-headline-medium;
    font-weight: 700;
    color: var(--md-primary);
  }
  &-label {
    @include md-label-medium;
    color: var(--md-on-surface-variant);
    margin-left: 4px;
  }
}
.gb-origin {
  color: var(--md-on-surface-variant);
  @include md-body-medium;
  text-decoration: line-through;
}
.gb-cap {
  color: var(--md-on-surface-variant);
  @include md-body-small;
}
.gb-open-btn {
  margin-top: 8px;
}

// 团列表
.gb-sec {
  margin-bottom: 10px;
}
.gb-sec-title {
  @include md-title-small;
  color: var(--md-on-surface-variant);
}
.gb-card {
  margin-bottom: 12px;
}
.gb-card-inner {
  padding: 14px 16px;
  display: flex;
  gap: 12px;
}
.gb-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.gb-leader-row {
  display: flex;
  align-items: center;
  gap: 10px;
}
.gb-leader-info {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}
.gb-leader {
  color: var(--md-on-surface);
  @include md-title-small;
}
.gb-end {
  color: var(--md-on-surface-variant);
  @include md-label-small;
}
.gb-progress {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.gb-progress-meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.gb-count {
  color: var(--md-on-surface-variant);
  @include md-label-medium;
}
.gb-lack {
  color: var(--md-tertiary);
  @include md-label-medium;
  font-weight: 500;
}
.gb-side {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  justify-content: space-between;
}
.gb-side-price {
  color: var(--md-primary);
  font-weight: 700;
  @include md-title-medium;
}
.gb-side-join {
  color: var(--md-primary);
  @include md-label-medium;
}
</style>
