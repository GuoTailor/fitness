<template>
  <view class="md-page">
    <M3TopAppBar title="领券中心" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 首屏骨架 -->
      <template v-if="loading && !items.length">
        <M3Card v-for="i in 3" :key="i" variant="filled" class="cp-skeleton">
          <view class="cp-skeleton-inner">
            <M3Skeleton variant="rect" width="104" height="88" />
            <view class="cp-skeleton-lines">
              <M3Skeleton variant="text" width="60%" height="14" />
              <M3Skeleton variant="text" width="80%" height="12" />
              <M3Skeleton variant="rect" width="72" height="28" />
            </view>
          </view>
        </M3Card>
      </template>

      <!-- 空态 -->
      <M3EmptyState
        v-else-if="!items.length"
        icon="redeem"
        title="暂无可领的券"
        description="新券上架后会第一时间出现在这里"
      />

      <!-- 券列表 -->
      <template v-else>
        <M3Card
          v-for="c in items"
          :key="c.templateId"
          variant="filled"
          class="cp-card"
        >
          <view class="cp-inner">
            <!-- 左侧：金额/折扣大字（按券类型区分容器色） -->
            <view class="cp-left" :class="'cp-left--' + (c.type || 'reduce')">
              <text class="cp-label">{{ c.label }}</text>
              <text v-if="thresholdText(c)" class="cp-threshold">{{ thresholdText(c) }}</text>
            </view>
            <!-- 右侧：信息 + 领取 -->
            <view class="cp-right">
              <view class="cp-name md-ellipsis">{{ c.name }}</view>
              <text class="cp-valid">领取后 {{ c.validDays }} 天内有效</text>
              <view class="cp-meta">
                <text class="cp-scope">{{ c.packageId ? '限指定课包' : '全场通用' }}</text>
                <text v-if="c.myReceived > 0" class="cp-received">已领 {{ c.myReceived }} 张</text>
              </view>
              <view class="cp-action">
                <M3Button
                  v-if="c.claimable"
                  size="sm"
                  variant="filled"
                  icon="redeem"
                  :loading="claimingId === c.templateId"
                  @click="onClaim(c)"
                >领取</M3Button>
                <M3Button v-else size="sm" variant="outlined" disabled>已领取</M3Button>
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
import { yuan } from '@/utils/format'

const auth = useAuthStore()
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

const items = ref([])
const loading = ref(true)
const claimingId = ref(0)

onLoad(async () => {
  await auth.ensureLogin().catch(() => {})
})

onShow(() => {
  load()
})

async function load() {
  loading.value = true
  try {
    const data = await request({ url: '/api/v1/coupons/center' })
    items.value = (data && data.items) || []
  } catch (e) {
    // 错误已由请求层 toast
  } finally {
    loading.value = false
  }
}

// 门槛提示（满减的门槛已在 label 里，折扣/赠课券需单独提示）
function thresholdText(c) {
  if (!c.threshold) return ''
  if (c.type === 'reduce') return ''
  return `满 ${yuan(c.threshold)} 元可用`
}

async function onClaim(c) {
  if (!c.claimable || claimingId.value) return
  claimingId.value = c.templateId
  try {
    await request({ url: `/api/v1/coupons/${c.templateId}/claim`, method: 'POST', data: {} })
    uni.showToast({ title: '领取成功', icon: 'success' })
    load() // 领取成功刷新（余量/限领状态变化）
  } catch (e) {
    // toast 已处理
  } finally {
    claimingId.value = 0
  }
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
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
}
.cp-inner {
  display: flex;
  gap: 14px;
  padding: 14px 16px;
}

// 左侧券面：primary-container=满减 / tertiary-container=折扣 / secondary-container=赠课
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
.cp-valid {
  color: var(--md-on-surface-variant);
  @include md-body-small;
}
.cp-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}
.cp-scope {
  padding: 1px 8px;
  border-radius: $md-shape-full;
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface-variant);
  @include md-label-small;
}
.cp-received {
  color: var(--md-on-surface-variant);
  @include md-label-small;
}
.cp-action {
  margin-top: 6px;
  display: flex;
  justify-content: flex-end;
}

.cp-more {
  padding: 8px 0;
}
</style>
