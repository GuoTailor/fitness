<template>
  <view class="md-page">
    <M3TopAppBar title="我的优惠券" :back="true" :elevated="scrolled">
      <template #actions>
        <M3IconButton icon="redeem" @click="goCenter" />
      </template>
    </M3TopAppBar>
    <view class="md-shell page-body">
      <!-- 状态 tab -->
      <M3Tabs :tabs="TABS" :model-value="tab" @update:model-value="onTab" class="cp-tabs" />

      <!-- 骨架屏 -->
      <template v-if="loading && !items.length">
        <M3Card v-for="i in 3" :key="i" variant="filled" class="cp-skeleton">
          <view class="cp-skeleton-inner">
            <M3Skeleton variant="rect" width="104" height="88" />
            <view class="cp-skeleton-lines">
              <M3Skeleton variant="text" width="55%" height="14" />
              <M3Skeleton variant="text" width="75%" height="12" />
            </view>
          </view>
        </M3Card>
      </template>

      <!-- 空态 -->
      <M3EmptyState
        v-else-if="!items.length"
        icon="confirmation_number"
        :title="tab === 'usable' ? '这里还没有券' : tab === 'used' ? '没有已使用的券' : '没有已过期的券'"
        :description="tab === 'usable' ? '去领券中心逛逛吧' : ''"
      >
        <M3Button v-if="tab === 'usable'" variant="filled" icon="redeem" @click="goCenter">去领券中心</M3Button>
      </M3EmptyState>

      <!-- 券列表 -->
      <template v-else>
        <M3Card
          v-for="c in items"
          :key="c.couponId"
          variant="filled"
          class="cp-card"
          :class="{ 'cp-card--off': c.status !== 'usable' }"
        >
          <view class="cp-inner">
            <view class="cp-left" :class="'cp-left--' + typeOf(c)">
              <text class="cp-label">{{ c.label || '优惠券' }}</text>
            </view>
            <view class="cp-right">
              <view class="cp-name md-ellipsis">{{ c.name }}</view>
              <text class="cp-expire">{{ c.expireText || c.statusText }}</text>
              <view class="cp-status" :style="{ color: statusColor(c.status) }">
                <view class="cp-status-dot" :style="{ background: statusColor(c.status) }" />
                <text class="cp-status-label">{{ statusText(c.status) }}</text>
              </view>
            </view>
          </view>
        </M3Card>

        <view v-if="loading" class="cp-more">
          <M3Progress type="linear" indeterminate />
        </view>
      </template>
    </view>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onShow, onPageScroll } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
import { fmtDate } from '@/utils/format'

const auth = useAuthStore()
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

// 状态 → 中文 + MD3 语义色
const STATUS = {
  usable: ['可使用', 'var(--md-primary)'],
  used: ['已使用', 'var(--md-outline)'],
  expired: ['已过期', 'var(--md-error)']
}
function statusText(s) {
  return (STATUS[s] || [s])[0]
}
function statusColor(s) {
  return (STATUS[s] || [s, 'var(--md-on-surface-variant)'])[1]
}

// mine 接口未返回 type，按 label 文案推断券面类型（折=折扣 / 课时=赠课 / 其余=满减）
function typeOf(c) {
  if (c.type) return c.type
  const label = c.label || ''
  if (label.includes('折')) return 'discount'
  if (label.includes('课时')) return 'gift'
  return 'reduce'
}

const TABS = [
  { value: 'usable', label: '可使用' },
  { value: 'used', label: '已使用' },
  { value: 'expired', label: '已过期' }
]

const tab = ref('usable')
const items = ref([])
const loading = ref(true)

onLoad(async () => {
  await auth.ensureLogin().catch(() => {})
})

onShow(() => {
  load()
})

function onTab(v) {
  if (v === tab.value) return
  tab.value = v
  load()
}

async function load() {
  loading.value = true
  try {
    const data = await request({ url: `/api/v1/coupons/mine?status=${tab.value}` })
    items.value = ((data && data.items) || []).map((x) => ({
      ...x,
      expireText: x.expireAt ? fmtDate(x.expireAt) + ' 前有效' : ''
    }))
  } catch (e) {
    // 错误已由请求层 toast
  } finally {
    loading.value = false
  }
}

function goCenter() {
  uni.navigateTo({ url: '/pages/coupon/center' })
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 0 16px 32px;
}

.cp-tabs {
  margin: 0 -16px 12px;
}

.cp-skeleton {
  margin-bottom: 12px;
}
.cp-skeleton-inner {
  padding: 16px;
  display: flex;
  gap: 14px;
}
.cp-skeleton-lines {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.cp-card {
  margin-bottom: 12px;
  transition: opacity $md-motion-short $md-motion-standard;
  // 过期/已用置灰
  &--off {
    opacity: 0.55;
  }
}
.cp-inner {
  display: flex;
  gap: 14px;
  padding: 14px 16px;
}

.cp-left {
  width: 104px;
  flex-shrink: 0;
  border-radius: $md-shape-md;
  padding: 14px 10px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  text-align: center;
  &--reduce {
    background: var(--md-primary-container);
    color: var(--md-on-primary-container);
  }
  &--discount {
    background: var(--md-tertiary-container);
    color: var(--md-on-tertiary-container);
  }
  &--gift {
    background: var(--md-secondary-container);
    color: var(--md-on-secondary-container);
  }
}
.cp-label {
  @include md-title-medium;
  font-weight: 700;
  line-height: 20px;
}
.cp-threshold {
  @include md-label-small;
  opacity: 0.8;
}

.cp-right {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.cp-name {
  color: var(--md-on-surface);
  @include md-title-small;
}
.cp-expire {
  color: var(--md-on-surface-variant);
  @include md-body-small;
}

// 状态：颜色圆点 + label-small
.cp-status {
  margin-top: 2px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  &-dot {
    width: 8px;
    height: 8px;
    border-radius: $md-shape-full;
  }
  &-label {
    @include md-label-small;
  }
}

.cp-more {
  padding: 8px 0;
}
</style>
