<template>
  <view class="md-page">
    <M3TopAppBar title="预约详情" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 骨架屏 -->
      <template v-if="loading">
        <M3Card variant="filled" class="sk-card">
          <view class="sk-inner">
            <M3Skeleton variant="text" width="45%" height="18" />
            <M3Skeleton variant="text" width="70%" height="14" />
            <M3Skeleton variant="rect" width="100%" height="96" />
          </view>
        </M3Card>
      </template>

      <template v-else>
        <!-- 核销码卡片 -->
        <M3Card variant="filled" class="code-card">
          <view class="code-head">
            <text class="code-title md-ellipsis">{{ title }}</text>
            <view v-if="statusText" class="st" :style="{ color: statusColor }">
              <view class="st-dot" :style="{ background: statusColor }" />
              <text class="st-label">{{ statusText }}</text>
            </view>
          </view>
          <view class="code-meta">
            <M3Icon name="schedule" :size="16" class="meta-icon" />
            <text class="meta-text">{{ timeText }}</text>
          </view>
          <view v-if="location" class="code-meta">
            <M3Icon name="location_on" :size="16" class="meta-icon" />
            <text class="meta-text">{{ location }}</text>
          </view>

          <!-- 核销码 -->
          <view v-if="code" class="code-box">
            <text class="code-value">{{ code }}</text>
            <text class="code-tip">到店向教练出示此码（或让教练输入）完成上课</text>
            <M3Button variant="tonal" icon="content_copy" size="sm" @click="onCopy">复制核销码</M3Button>
          </view>
          <view v-else class="code-box code-box--empty">
            <M3Icon name="info" :size="20" class="empty-icon" />
            <text class="code-tip">当前状态（{{ statusText || '未知' }}）无需核销</text>
          </view>
        </M3Card>

        <!-- 教练课后反馈 -->
        <M3Card v-if="feedback" variant="outlined" class="fb-card">
          <view class="fb-head">
            <text class="fb-title">教练课后反馈</text>
            <text class="fb-time">{{ feedback.createdAt }}</text>
          </view>
          <text class="fb-content">{{ feedback.content }}</text>
        </M3Card>

        <!-- 课后评价入口 -->
        <M3Card v-if="canReview" variant="outlined" class="review-card">
          <view class="review-inner">
            <M3Icon name="reviews" :size="28" class="review-icon" />
            <view class="review-text">
              <text class="review-title">这节课已完成</text>
              <text class="review-sub">说说你的上课体验吧</text>
            </view>
            <M3Button variant="filled" size="sm" @click="goReview">去评价</M3Button>
          </view>
        </M3Card>
      </template>
    </view>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onPageScroll } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
import { weekday } from '@/utils/format'

const auth = useAuthStore()

// 状态 → MD3 语义色
const STATUS = {
  pending: ['待确认', 'var(--md-tertiary)'],
  confirmed: ['已确认', 'var(--md-primary)'],
  completed: ['已完成', 'var(--md-on-surface-variant)'],
  cancelled: ['已取消', 'var(--md-outline)'],
  expired: ['已过期', 'var(--md-outline)'],
  enrolling: ['报名中', 'var(--md-tertiary)'],
  enrolled: ['已报名', 'var(--md-primary)'],
  verified: ['已核销', 'var(--md-primary)'],
  quit: ['已退出', 'var(--md-outline)']
}

const loading = ref(true)
const scrolled = ref(false)
const id = ref(0)
const type = ref('private')

const title = ref('')
const timeText = ref('')
const location = ref('')
const statusText = ref('')
const statusColor = ref('')
const code = ref('')
const canReview = ref(false)
const feedback = ref(null)

onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

onLoad((query) => {
  id.value = Number(query.id) || 0
  type.value = query.type === 'group' ? 'group' : 'private'
  load()
})

function fmtMinute(m) {
  return `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`
}

function applyStatus(s) {
  const [text, color] = STATUS[s] || [s, 'var(--md-on-surface-variant)']
  statusText.value = text
  statusColor.value = color
}

async function load() {
  try {
    await auth.ensureLogin()
  } catch (e) { return } // H5 未登录已跳登录页
  loading.value = true
  code.value = ''
  feedback.value = null
  canReview.value = false
  let canCode = false
  try {
    if (type.value === 'group') {
      // 团课：GET /api/v1/group-classes/{id} → { class, myStatus, feedback? }
      const d = await request({ url: `/api/v1/group-classes/${id.value}` })
      const c = d.class || {}
      title.value = c.title || '团课'
      timeText.value = `${c.classDate} ${fmtMinute(c.startMinute)}-${fmtMinute(c.endMinute)}`
      location.value = c.location || ''
      canCode = d.myStatus === 'enrolled'
      canReview.value = d.myStatus === 'verified'
      feedback.value = d.feedback || null
      applyStatus(d.myStatus || c.status)
    } else {
      // 私教：GET /api/v1/bookings/{id} → { booking, feedback? }
      const d = await request({ url: `/api/v1/bookings/${id.value}` })
      const b = d.booking || {}
      title.value = '私教课'
      timeText.value = `${b.bookingDate} ${weekday(b.bookingDate)} ${fmtMinute(b.startMinute)}-${fmtMinute(b.endMinute)}`
      location.value = ''
      canCode = b.status === 'pending' || b.status === 'confirmed'
      canReview.value = b.status === 'completed'
      feedback.value = d.feedback || null
      applyStatus(b.status)
    }
    if (feedback.value && feedback.value.createdAt) {
      feedback.value.createdAt = String(feedback.value.createdAt).replace('T', ' ').slice(0, 16)
    }
    if (canCode) {
      const url = type.value === 'group'
        ? `/api/v1/group-classes/${id.value}/code`
        : `/api/v1/bookings/${id.value}/code`
      const v = await request({ url })
      code.value = (v && v.code) || ''
    }
  } catch (e) { /* 错误 toast 已由请求层处理 */ } finally {
    loading.value = false
  }
}

function onCopy() {
  uni.setClipboardData({
    data: code.value,
    success() {
      // #ifdef H5
      uni.showToast({ title: '已复制', icon: 'none' })
      // #endif
      // MP 端 setClipboardData 自带「内容已复制」提示
    }
  })
}

function goReview() {
  uni.navigateTo({ url: '/pages/review/create' })
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

.code-card {
  margin-bottom: 12px;
  padding: 16px;
}
.code-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.code-title {
  flex: 1;
  min-width: 0;
  color: var(--md-on-surface);
  @include md-title-large;
}
.code-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
}
.meta-icon {
  color: var(--md-on-surface-variant);
  flex-shrink: 0;
}
.meta-text {
  color: var(--md-on-surface-variant);
  @include md-body-medium;
}

.code-box {
  margin-top: 16px;
  padding: 20px 16px;
  border-radius: $md-shape-md;
  background: var(--md-surface-container-lowest);
  border: 1px dashed var(--md-outline-variant);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  &--empty {
    border-style: solid;
    flex-direction: row;
    justify-content: center;
    gap: 8px;
    padding: 14px 16px;
  }
}
.code-value {
  color: var(--md-primary);
  @include md-headline-medium;
  font-weight: 600;
  letter-spacing: 6px;
}
.code-tip {
  color: var(--md-on-surface-variant);
  @include md-body-small;
  text-align: center;
}
.empty-icon {
  color: var(--md-on-surface-variant);
  flex-shrink: 0;
}

.fb-card,
.review-card {
  margin-bottom: 12px;
}
.fb-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}
.fb-title {
  color: var(--md-on-surface);
  @include md-title-medium;
}
.fb-time {
  color: var(--md-on-surface-variant);
  @include md-body-small;
}
.fb-content {
  display: block;
  margin-top: 8px;
  color: var(--md-on-surface-variant);
  @include md-body-medium;
  line-height: 22px;
}

.review-inner {
  padding: 16px;
  display: flex;
  align-items: center;
  gap: 12px;
}
.review-icon {
  color: var(--md-primary);
  flex-shrink: 0;
}
.review-text {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.review-title {
  color: var(--md-on-surface);
  @include md-title-small;
}
.review-sub {
  color: var(--md-on-surface-variant);
  @include md-body-small;
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
