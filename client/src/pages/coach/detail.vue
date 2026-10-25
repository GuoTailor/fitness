<template>
  <view class="md-page">
    <M3TopAppBar :title="detail ? detail.coach.name : '教练主页'" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 加载骨架 -->
      <template v-if="!detail">
        <view v-if="loadFailed" class="load-failed">
          <M3EmptyState icon="person" title="教练不存在或已停用">
            <M3Button variant="tonal" @click="goCoachList">看看其他教练</M3Button>
          </M3EmptyState>
        </view>
        <template v-else>
          <view class="sk-head">
            <M3Skeleton variant="circle" :width="72" :height="72" />
            <view class="sk-head__lines">
              <M3Skeleton variant="text" width="40%" />
              <M3Skeleton variant="text" width="70%" />
            </view>
          </view>
          <M3Skeleton variant="rect" height="120px" class="sk-block" />
          <M3Skeleton variant="rect" height="180px" class="sk-block" />
        </template>
      </template>

      <!-- 教练主页 -->
      <template v-else>
        <!-- 头部卡片 -->
        <M3Card variant="filled" class="head-card">
          <view class="head-card__row">
            <M3Avatar :src="detail.coach.avatarUrl" :name="detail.coach.name" :size="72" />
            <view class="head-card__info">
              <view class="head-card__name-line">
                <text class="head-card__name">{{ detail.coach.name }}</text>
                <view v-if="detail.coach.certVerified" class="head-card__cert">
                  <M3Icon name="verified" :size="12" />
                  <text>资质已认证</text>
                </view>
              </view>
              <text class="head-card__sub">
                {{ detail.coach.specialties || '综合训练' }} · {{ detail.coach.years }}年教龄
              </text>
              <view v-if="detail.coach.ratingCount" class="head-card__rating">
                <M3Stars :model-value="Number(detail.coach.ratingAvg) || 0" :size="14" show-value />
                <text class="head-card__rating-count">（{{ detail.coach.ratingCount }}条评价）</text>
              </view>
            </view>
          </view>
        </M3Card>

        <!-- 介绍视频 -->
        <video
          v-if="detail.coach.introVideoUrl"
          class="intro-video"
          :src="detail.coach.introVideoUrl"
          controls
          object-fit="cover"
        />

        <!-- 自我介绍 -->
        <M3Card v-if="detail.coach.intro" variant="outlined" class="section-card">
          <view class="section-card__title">自我介绍</view>
          <text class="section-card__rich">{{ detail.coach.intro }}</text>
        </M3Card>

        <!-- 可约时段 -->
        <M3Card v-if="scheduleText" variant="outlined" class="section-card">
          <view class="section-card__title">可约时段（每周模板）</view>
          <text class="section-card__schedule">{{ scheduleText }}</text>
          <view class="section-card__hint">
            <M3Icon name="schedule" :size="14" />
            <text>支持 30 分钟粒度预约，以下单页实时可约为准</text>
          </view>
        </M3Card>

        <!-- 在售课包 -->
        <M3Card variant="outlined" class="section-card">
          <view class="section-card__title">在售课包</view>
          <template v-if="detail.packages && detail.packages.length">
            <view
              v-for="(p, i) in detail.packages"
              :key="p.id"
              class="pkg"
              :class="{ 'is-divided': i > 0 }"
              @click="onPkg(p.id)"
            >
              <view class="pkg__info">
                <text class="pkg__title md-ellipsis">{{ p.title }}</text>
                <text v-if="p.tags" class="pkg__sub md-ellipsis">{{ p.tags }}</text>
              </view>
              <view class="pkg__right">
                <text class="pkg__price">¥{{ yuan(p.minPrice) }}起</text>
                <view class="pkg__go">
                  <text>查看</text>
                  <M3Icon name="chevron_right" :size="16" />
                </view>
              </view>
            </view>
          </template>
          <text v-else class="inline-empty">暂无在售课包</text>
        </M3Card>

        <!-- 学员评价 -->
        <M3Card variant="outlined" class="section-card">
          <view class="section-card__title">学员评价（{{ detail.coach.ratingCount || 0 }}）</view>
          <template v-if="detail.reviews && detail.reviews.length">
            <view
              v-for="(r, i) in detail.reviews"
              :key="r.id"
              class="review"
              :class="{ 'is-divided': i > 0 }"
            >
              <view class="review__head">
                <M3Stars :model-value="Number(r.rating) || 0" :size="14" show-value />
                <text class="review__time">{{ fmtDate(r.createdAt) }}</text>
              </view>
              <text v-if="r.content" class="review__content">{{ r.content }}</text>
              <view v-if="r.coachReply" class="review__reply">
                <text class="review__reply-label">教练回复</text>
                <text class="review__reply-text">{{ r.coachReply }}</text>
              </view>
            </view>
          </template>
          <text v-else class="inline-empty">暂无评价</text>
          <view
            v-if="detail.coach.ratingCount > 5"
            class="all-reviews"
            @click="onAllReviews"
          >
            <text>查看全部 {{ detail.coach.ratingCount }} 条评价</text>
            <M3Icon name="chevron_right" :size="16" />
          </view>
        </M3Card>

        <!-- 底部占位：给固定预约栏留空间 -->
        <view class="footer-space" />
      </template>
    </view>

    <!-- 固定预约栏 -->
    <view v-if="detail" class="footer-bar">
      <view class="md-shell footer-bar__inner">
        <M3Button block size="lg" @click="onBook">预约 TA 的课</M3Button>
      </view>
    </view>
  </view>
</template>

<script setup>
import { computed, ref } from 'vue'
import { onLoad, onPageScroll } from '@dcloudio/uni-app'
import { useAuthStore } from '@/stores/auth'
import { request } from '@/utils/request'
import { yuan, fmtDate } from '@/utils/format'

const auth = useAuthStore()

const id = ref(0)
const detail = ref(null)
const loadFailed = ref(false)
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

onLoad(async (query) => {
  id.value = Number((query && query.id) || 0)
  // 小程序端静默登录（H5 为公开页，不强制登录）
  // #ifdef MP-WEIXIN
  await auth.ensureLogin().catch(() => {})
  // #endif
  load()
})

// 每周可约模板 → 可读文本：周一 09:00-12:00　周三 14:00-18:00
const scheduleText = computed(() => {
  const list = (detail.value && detail.value.schedules) || []
  return list
    .map((s) => `周${'日一二三四五六'[s.weekday]} ${fmtMinute(s.startMinute)}-${fmtMinute(s.endMinute)}`)
    .join('　')
})

function fmtMinute(m) {
  const h = Math.floor(m / 60)
  const mm = m % 60
  return `${h}:${String(mm).padStart(2, '0')}`
}

async function load() {
  loadFailed.value = false
  try {
    detail.value = await request({ url: `/api/v1/coaches/${id.value}` })
  } catch (e) {
    loadFailed.value = true
  }
}

function onPkg(pkgId) {
  uni.navigateTo({ url: '/pages/course/detail?id=' + pkgId })
}

function onAllReviews() {
  const c = detail.value.coach
  uni.navigateTo({ url: `/pages/review/public?type=coach&id=${c.id}` })
}

function onBook() {
  const c = detail.value.coach
  uni.navigateTo({
    url: `/pages/booking/private?coachId=${c.id}&coachName=${encodeURIComponent(c.name)}`
  })
}

function goCoachList() {
  uni.navigateBack()
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

// 骨架
.sk-head {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 4px 0 16px;
  &__lines {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
}
.sk-block {
  margin-top: 12px;
}
.load-failed {
  padding-top: 48px;
}

// 头部
.head-card {
  &__row {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 16px;
  }
  &__info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  &__name-line {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }
  &__name {
    @include md-headline-small;
    color: var(--md-on-surface);
    flex-shrink: 0;
  }
  &__cert {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    padding: 2px 8px;
    border-radius: $md-shape-xs;
    background: var(--md-primary-container);
    color: var(--md-on-primary-container);
    @include md-label-small;
    flex-shrink: 0;
  }
  &__sub {
    @include md-body-medium;
    color: var(--md-on-surface-variant);
  }
  &__rating {
    display: flex;
    align-items: center;
  }
  &__rating-count {
    @include md-label-small;
    color: var(--md-on-surface-variant);
  }
}

.intro-video {
  width: 100%;
  height: 210px;
  margin-top: 12px;
  border-radius: $md-shape-md;
  background: var(--md-inverse-surface);
}

// 分区卡片
.section-card {
  margin-top: 12px;
  padding: 16px;
  &__title {
    @include md-title-medium;
    color: var(--md-on-surface);
    margin-bottom: 12px;
  }
  &__rich {
    display: block;
    @include md-body-medium;
    color: var(--md-on-surface-variant);
    line-height: 1.7;
    white-space: pre-wrap;
    word-break: break-all;
  }
  &__schedule {
    display: block;
    @include md-body-medium;
    color: var(--md-on-surface-variant);
    line-height: 1.8;
  }
  &__hint {
    display: flex;
    align-items: center;
    gap: 4px;
    margin-top: 10px;
    color: var(--md-outline);
    @include md-body-small;
  }
}

// 课包行
.pkg {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0;
  cursor: pointer;
  &.is-divided {
    border-top: 1px solid var(--md-outline-variant);
  }
  &:active {
    opacity: 0.7;
  }
  &__info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  &__title {
    @include md-body-large;
    color: var(--md-on-surface);
  }
  &__sub {
    @include md-body-small;
    color: var(--md-on-surface-variant);
  }
  &__right {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 2px;
    flex-shrink: 0;
  }
  &__price {
    @include md-title-medium;
    color: var(--md-primary);
    font-weight: 600;
  }
  &__go {
    display: flex;
    align-items: center;
    color: var(--md-primary);
    @include md-label-small;
  }
}

// 评价
.review {
  padding: 12px 0;
  &.is-divided {
    border-top: 1px solid var(--md-outline-variant);
  }
  &__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }
  &__time {
    @include md-label-small;
    color: var(--md-outline);
  }
  &__content {
    display: block;
    margin-top: 6px;
    @include md-body-medium;
    color: var(--md-on-surface-variant);
    line-height: 1.6;
  }
  &__reply {
    margin-top: 8px;
    padding: 8px 12px;
    border-radius: $md-shape-sm;
    background: var(--md-surface-container-high);
  }
  &__reply-label {
    display: block;
    @include md-label-small;
    color: var(--md-primary);
    margin-bottom: 2px;
  }
  &__reply-text {
    @include md-body-small;
    color: var(--md-on-surface-variant);
    line-height: 1.6;
  }
}

.all-reviews {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 2px;
  padding: 12px 0 0;
  color: var(--md-primary);
  @include md-label-large;
  cursor: pointer;
  &:active {
    opacity: 0.7;
  }
}

.inline-empty {
  display: block;
  text-align: center;
  padding: 16px 0;
  @include md-body-small;
  color: var(--md-outline);
}

// 固定预约栏
.footer-space {
  height: 88px;
}
.footer-bar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 90;
  background: var(--md-surface-container);
  padding: 12px 16px calc(12px + env(safe-area-inset-bottom));
  @include md-elevation-1;
  &__inner {
    padding: 0;
  }
}
</style>
