<template>
  <view class="md-page">
    <M3TopAppBar title="我的预约" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 骨架屏 -->
      <template v-if="loading && !list.length">
        <M3Card v-for="i in 3" :key="i" variant="filled" class="sk-card">
          <view class="sk-inner">
            <view class="sk-row">
              <M3Skeleton variant="text" width="55%" height="16" />
              <M3Skeleton variant="text" width="48px" height="14" />
            </view>
            <M3Skeleton variant="text" width="35%" height="14" />
          </view>
        </M3Card>
      </template>

      <!-- 空态 -->
      <M3EmptyState
        v-else-if="!list.length"
        icon="event_available"
        title="暂无预约"
        description="挑选一节私教课，随时开始训练"
      >
        <M3Button variant="filled" icon="fitness_center" @click="goBooking">去预约</M3Button>
      </M3EmptyState>

      <!-- 列表 -->
      <template v-else>
        <M3Card v-for="b in list" :key="b.id" variant="filled" class="bk-card">
          <view class="bk-main" @click="goDetail(b)">
            <view class="bk-head">
              <view class="bk-time">
                <M3Icon name="schedule" :size="16" class="bk-time-icon" />
                <text class="bk-time-text md-ellipsis">{{ b.timeText }}</text>
              </view>
              <view class="st" :style="{ color: b.statusColor }">
                <view class="st-dot" :style="{ background: b.statusColor }" />
                <text class="st-label">{{ b.statusText }}</text>
              </view>
            </view>
            <view class="bk-sub">
              <text class="bk-type">私教课</text>
              <M3Icon name="chevron_right" :size="18" class="bk-go" />
            </view>
          </view>
          <view v-if="canAct(b)" class="bk-actions">
            <M3Button size="sm" variant="outlined" danger @click="askCancel(b)">取消预约</M3Button>
            <M3Button size="sm" variant="tonal" icon="qr_code" @click="goDetail(b)">出示核销码</M3Button>
          </view>
        </M3Card>
      </template>
    </view>

    <!-- 取消确认 -->
    <M3Dialog :open="!!cancelId" title="取消预约" icon="warning" :dismissible="false">
      <text>开课前 4 小时内取消将扣 1 课时，确认取消本次预约？</text>
      <template #actions>
        <M3Button variant="text" @click="cancelId = 0">再想想</M3Button>
        <M3Button variant="filled" danger :loading="cancelling" @click="doCancel">确认取消</M3Button>
      </template>
    </M3Dialog>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onShow, onPageScroll } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
import { weekday } from '@/utils/format'

const auth = useAuthStore()

// 状态 → MD3 语义色（颜色圆点 + label-small，不用大色块）
const STATUS = {
  pending: ['待确认', 'var(--md-tertiary)'],
  confirmed: ['已确认', 'var(--md-primary)'],
  completed: ['已完成', 'var(--md-on-surface-variant)'],
  cancelled: ['已取消', 'var(--md-outline)'],
  expired: ['已过期', 'var(--md-outline)']
}

const list = ref([])
const loading = ref(false)
const scrolled = ref(false)
const cancelId = ref(0)
const cancelling = ref(false)

onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

onShow(() => { load() })

function fmtMinute(m) {
  return `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`
}

async function load() {
  try {
    await auth.ensureLogin()
  } catch (e) { return } // H5 未登录已跳登录页
  loading.value = true
  try {
    const data = (await request({ url: '/api/v1/bookings' })) || []
    list.value = data.map((b) => {
      const [statusText, statusColor] = STATUS[b.status] || [b.status, 'var(--md-on-surface-variant)']
      return {
        ...b,
        statusText,
        statusColor,
        timeText: `${b.bookingDate} ${weekday(b.bookingDate)} ${fmtMinute(b.startMinute)}-${fmtMinute(b.endMinute)}`
      }
    })
  } catch (e) { /* 错误 toast 已由请求层处理 */ } finally {
    loading.value = false
  }
}

function canAct(b) {
  return b.status === 'pending' || b.status === 'confirmed'
}

function goBooking() {
  uni.reLaunch({ url: '/pages/booking/private' })
}

function goDetail(b) {
  uni.navigateTo({ url: `/pages/booking/detail?id=${b.id}&type=private` })
}

function askCancel(b) {
  cancelId.value = b.id
}

async function doCancel() {
  cancelling.value = true
  try {
    await request({ url: `/api/v1/bookings/${cancelId.value}/cancel`, method: 'POST', data: {} })
    cancelId.value = 0
    uni.showToast({ title: '已取消', icon: 'success' })
    await load()
  } catch (e) { /* toast 已处理 */ } finally {
    cancelling.value = false
  }
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
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

.bk-card {
  margin-bottom: 12px;
}
.bk-main {
  padding: 14px 16px;
  cursor: pointer;
  &:active {
    background: rgba(0, 0, 0, $md-state-pressed-opacity);
  }
}
.bk-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.bk-time {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  flex: 1;
  &-icon {
    color: var(--md-primary);
    flex-shrink: 0;
  }
  &-text {
    color: var(--md-on-surface);
    @include md-title-medium;
  }
}
.bk-sub {
  margin-top: 8px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.bk-type {
  color: var(--md-on-surface-variant);
  @include md-body-medium;
}
.bk-go {
  color: var(--md-on-surface-variant);
}
.bk-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 0 16px 14px;
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
