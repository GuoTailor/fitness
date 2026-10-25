<template>
  <view class="md-page">
    <M3TopAppBar title="我的评价" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 骨架屏 -->
      <template v-if="loading && !items.length">
        <M3Card v-for="i in 3" :key="i" variant="filled" class="rv-skeleton">
          <view class="rv-skeleton-inner">
            <M3Skeleton variant="text" width="40%" height="14" />
            <M3Skeleton variant="text" width="90%" height="12" />
            <M3Skeleton variant="text" width="65%" height="12" />
          </view>
        </M3Card>
      </template>

      <!-- 空态 -->
      <M3EmptyState
        v-else-if="!items.length"
        icon="reviews"
        title="还没有评价"
        description="完成上课并核销后来评价吧"
      >
        <M3Button variant="filled" icon="edit" @click="goCreate">去评价完成的课程</M3Button>
      </M3EmptyState>

      <!-- 评价列表 -->
      <template v-else>
        <M3Card v-for="(it, idx) in items" :key="idx" variant="filled" class="rv-card">
          <view class="rv-card-inner">
            <view class="rv-head">
              <text class="rv-type">{{ it.typeText }}</text>
              <view class="st" :style="{ color: it.statusColor }">
                <view class="st-dot" :style="{ background: it.statusColor }" />
                <text class="st-label">{{ it.statusText }}</text>
              </view>
            </view>
            <M3Stars :model-value="it.review.rating" :size="16" show-value class="rv-stars" />
            <text v-if="it.review.content" class="rv-content">{{ it.review.content }}</text>
            <view v-if="it.images && it.images.length" class="rv-img-grid">
              <image
                v-for="(img, i) in it.images"
                :key="i"
                :src="img"
                mode="aspectFill"
                class="rv-img"
                @click="onPreview(it, i)"
              />
            </view>
            <view v-if="it.review.coachReply" class="rv-reply">
              <text class="rv-reply-text">教练回复：{{ it.review.coachReply }}</text>
            </view>
            <text class="rv-time">{{ it.timeText }}</text>
          </view>
        </M3Card>
      </template>
    </view>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onShow, onPageScroll } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
import { fmtTime } from '@/utils/format'

const auth = useAuthStore()
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

// 审核状态 → 中文 + MD3 语义色
const STATUS = {
  pending: ['审核中', 'var(--md-tertiary)'],
  approved: ['已通过', 'var(--md-primary)'],
  hidden: ['已隐藏', 'var(--md-error)']
}

const items = ref([])
const loading = ref(true)

onLoad(async () => {
  await auth.ensureLogin().catch(() => {})
})

onShow(() => {
  load()
})

async function load() {
  loading.value = true
  try {
    const rows = (await request({ url: '/api/v1/reviews/mine' })) || []
    items.value = rows.map((x) => {
      const [statusText, statusColor] = STATUS[x.review.status] || [x.review.status, 'var(--md-on-surface-variant)']
      return {
        ...x,
        statusText,
        statusColor,
        typeText: x.targetType === 'coach' ? '教练服务' : '课程体验',
        timeText: fmtTime(x.review.createdAt)
      }
    })
  } catch (e) {
    // 错误已由请求层 toast
  } finally {
    loading.value = false
  }
}

function onPreview(it, i) {
  uni.previewImage({ current: it.images[i], urls: it.images })
}

function goCreate() {
  uni.navigateTo({ url: '/pages/review/create' })
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

.rv-skeleton {
  margin-bottom: 12px;
}
.rv-skeleton-inner {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.rv-card {
  margin-bottom: 12px;
}
.rv-card-inner {
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.rv-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.rv-type {
  color: var(--md-on-surface);
  @include md-title-small;
}
.rv-stars {
  align-self: flex-start;
}
.rv-content {
  color: var(--md-on-surface-variant);
  @include md-body-medium;
  line-height: 22px;
  word-break: break-all;
}

.rv-img-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.rv-img {
  width: 88px;
  height: 88px;
  border-radius: $md-shape-sm;
  background: var(--md-surface-container-highest);
}

.rv-reply {
  padding: 10px 12px;
  border-radius: $md-shape-sm;
  background: var(--md-primary-container);
}
.rv-reply-text {
  color: var(--md-on-primary-container);
  @include md-body-small;
  line-height: 20px;
  word-break: break-all;
}
.rv-time {
  align-self: flex-end;
  color: var(--md-on-surface-variant);
  @include md-label-small;
}

// 审核状态：颜色圆点 + label-small
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
