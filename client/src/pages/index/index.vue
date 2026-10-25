<template>
  <view class="md-page">
    <M3TopAppBar title="健身工作室">
      <template #actions>
        <!-- #ifdef H5 -->
        <M3IconButton icon="share" @click="onShareH5" />
        <!-- #endif -->
      </template>
    </M3TopAppBar>

    <view class="md-shell page-body">
      <!-- 骨架屏 -->
      <template v-if="loading && !loaded">
        <M3Skeleton variant="rect" height="150px" />
        <M3Card variant="filled" class="skeleton-entries">
          <view class="entries-row">
            <view v-for="i in 4" :key="i" class="entry-skeleton">
              <M3Skeleton variant="circle" width="48" height="48" />
              <M3Skeleton variant="text" width="48" height="12" />
            </view>
          </view>
        </M3Card>
        <view class="pkg-grid">
          <M3Card v-for="i in 4" :key="i" variant="filled" class="pkg-card">
            <M3Skeleton variant="rect" height="100px" />
            <view class="skeleton-lines">
              <M3Skeleton variant="text" height="14" />
              <M3Skeleton variant="text" width="80" height="14" />
            </view>
          </M3Card>
        </view>
      </template>

      <!-- 空态 -->
      <M3EmptyState
        v-else-if="!banners.length && !hotPackages.length && !coaches.length"
        icon="fitness_center"
        title="首页暂无内容"
        description="先去课包商城看看吧"
      >
        <M3Button variant="filled" @click="goCourses">去逛逛</M3Button>
      </M3EmptyState>

      <template v-else>
        <!-- Banner 轮播 -->
        <swiper
          v-if="banners.length"
          class="banner"
          circular
          autoplay
          :interval="4000"
          :duration="400"
          indicator-dots
          indicator-color="var(--md-surface-container-highest)"
          indicator-active-color="var(--md-primary)"
        >
          <swiper-item v-for="b in banners" :key="b.id" @click="onBannerTap(b.linkType, b.linkValue)">
            <image class="banner-img" :src="b.imageUrl" mode="aspectFill" />
          </swiper-item>
        </swiper>

        <!-- 功能入口 -->
        <M3Card variant="filled" class="entries">
          <view class="entries-row">
            <view class="entry" @click="goCourses">
              <view class="entry-icon is-primary"><M3Icon name="shopping_bag" :size="24" /></view>
              <text class="entry-label">课包商城</text>
            </view>
            <view class="entry" @click="goCalendar">
              <view class="entry-icon is-tertiary"><M3Icon name="calendar_month" :size="24" /></view>
              <text class="entry-label">团课课表</text>
            </view>
            <view class="entry" @click="goCoaches">
              <view class="entry-icon is-secondary"><M3Icon name="fitness_center" :size="24" /></view>
              <text class="entry-label">找教练</text>
            </view>
            <view class="entry" @click="goBookings">
              <view class="entry-icon is-primary"><M3Icon name="event_available" :size="24" /></view>
              <text class="entry-label">我的预约</text>
            </view>
          </view>
        </M3Card>

        <!-- 热销课包 -->
        <view v-if="hotPackages.length" class="section">
          <view class="section-title">
            <M3Icon name="local_fire_department" :size="20" class="section-title-icon" />
            <text>热销课包</text>
          </view>
          <view class="pkg-grid">
            <M3Card
              v-for="p in hotPackages"
              :key="p.id"
              variant="filled"
              clickable
              class="pkg-card"
              @click="goPkg(p.id)"
            >
              <image v-if="p.coverUrl" class="pkg-cover" :src="p.coverUrl" mode="aspectFill" />
              <view v-else class="pkg-cover pkg-cover--ph">
                <M3Icon name="fitness_center" :size="36" />
              </view>
              <view class="pkg-body">
                <view class="pkg-name md-ellipsis">{{ p.title }}</view>
                <view class="pkg-meta">
                  <text class="pkg-price">¥{{ yuan(p.minPrice) }}</text>
                  <text class="pkg-sold">已售 {{ p.soldCount }}</text>
                </view>
              </view>
            </M3Card>
          </view>
        </view>

        <!-- 明星教练 -->
        <view v-if="coaches.length" class="section">
          <view class="section-title">
            <M3Icon name="workspace_premium" :size="20" class="section-title-icon" />
            <text>明星教练</text>
          </view>
          <scroll-view scroll-x class="coach-scroll">
            <view
              v-for="c in coaches"
              :key="c.id"
              class="coach-card"
              @click="goCoach(c.id)"
            >
              <M3Avatar :src="c.avatarUrl" :name="c.name" :size="48" />
              <text class="coach-name md-ellipsis">{{ c.name }}</text>
              <text class="coach-sub md-ellipsis">{{ c.specialties }}</text>
            </view>
          </scroll-view>
        </view>
      </template>
    </view>

    <M3BottomNavigation :items="TABS" model-value="home" @change="onTab" />
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onShow, onShareAppMessage } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
import { yuan } from '@/utils/format'
// #ifdef MP-WEIXIN
import { mpSharePayload } from '@/utils/referral'
// #endif
// #ifdef H5
import { shareLink } from '@/utils/referral'
// #endif

const auth = useAuthStore()

const TABS = [
  { key: 'home', icon: 'home', label: '首页' },
  { key: 'course', icon: 'shopping_bag', label: '课包' },
  { key: 'mine', icon: 'person', label: '我的' }
]

const banners = ref([])
const hotPackages = ref([])
const coaches = ref([])
const loading = ref(false)
const loaded = ref(false)

onLoad(async () => {
  // 登录态：MP 静默登录；H5 未登录跳登录页（throw 已 catch）
  await auth.ensureLogin().catch(() => {})
})

onShow(() => {
  loadHome()
})

// #ifdef MP-WEIXIN
// 老带新：转发卡片带上邀请码
onShareAppMessage(() => mpSharePayload('/pages/index/index', auth.user?.id || 0))
// #endif

// #ifdef H5
function onShareH5() {
  shareLink('/pages/index/index', auth.user?.id || 0)
}
// #endif

async function loadHome() {
  loading.value = true
  try {
    const data = await request({ url: '/api/v1/home', silent: true })
    banners.value = data.banners || []
    hotPackages.value = data.hotPackages || []
    coaches.value = data.coaches || []
    loaded.value = true
  } catch (e) {
    // 弱网容忍：静默，保留已有内容
  } finally {
    loading.value = false
  }
}

// Banner 跳转映射（与原页面一致）
function onBannerTap(type, value) {
  if (!value) return
  if (type === 'package') uni.navigateTo({ url: '/pages/course/detail?id=' + value })
  else if (type === 'coach') uni.navigateTo({ url: '/pages/coach/detail?id=' + value })
  else if (type === 'groupbuy') uni.navigateTo({ url: '/pages/groupbuy/list?activityId=' + value })
}

function goPkg(id) {
  uni.navigateTo({ url: '/pages/course/detail?id=' + id })
}
function goCoach(id) {
  uni.navigateTo({ url: '/pages/coach/detail?id=' + id })
}
// tab 页跳转一律 reLaunch（无原生 tabBar）
function goCourses() {
  uni.reLaunch({ url: '/pages/course/list' })
}
function goCoaches() {
  uni.navigateTo({ url: '/pages/coach/list' })
}
function goCalendar() {
  uni.navigateTo({ url: '/pages/booking/calendar' })
}
function goBookings() {
  uni.navigateTo({ url: '/pages/booking/list' })
}
function onTab(t) {
  uni.reLaunch({
    url: t.key === 'home' ? '/pages/index/index' : t.key === 'course' ? '/pages/course/list' : '/pages/mine/mine'
  })
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

.banner {
  height: 150px;
  border-radius: $md-shape-lg;
  overflow: hidden;

  .banner-img {
    width: 100%;
    height: 100%;
  }
}

.entries {
  margin-top: 12px;
  padding: 16px 0;
}

.entries-row {
  display: flex;
}

.entry {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  cursor: pointer;

  &:active {
    opacity: 0.7;
  }
}

.entry-icon {
  width: 48px;
  height: 48px;
  border-radius: $md-shape-lg;
  display: flex;
  align-items: center;
  justify-content: center;

  &.is-primary {
    background: var(--md-primary-container);
    color: var(--md-on-primary-container);
  }
  &.is-secondary {
    background: var(--md-secondary-container);
    color: var(--md-on-secondary-container);
  }
  &.is-tertiary {
    background: var(--md-tertiary-container);
    color: var(--md-on-tertiary-container);
  }
}

.entry-label {
  @include md-label-medium;
  color: var(--md-on-surface);
}

.section {
  margin-top: 20px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
  @include md-title-medium;
  color: var(--md-on-surface);
}

.section-title-icon {
  color: var(--md-primary);
}

.pkg-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.pkg-card {
  width: calc(50% - 6px);
}

.pkg-cover {
  width: 100%;
  height: 100px;
  display: block;
  background: var(--md-surface-container-highest);
}

.pkg-cover--ph {
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--md-outline);
}

.pkg-body {
  padding: 8px 12px 12px;
}

.pkg-name {
  @include md-body-medium;
  color: var(--md-on-surface);
}

.pkg-meta {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-top: 4px;
}

.pkg-price {
  @include md-title-medium;
  color: var(--md-primary);
}

.pkg-sold {
  @include md-label-small;
  color: var(--md-on-surface-variant);
}

.coach-scroll {
  white-space: nowrap;
}

.coach-card {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  width: 104px;
  padding: 14px 8px;
  margin-right: 12px;
  border-radius: $md-shape-md;
  background: var(--md-surface-container-high);
  cursor: pointer;
  vertical-align: top;

  &:active {
    transform: scale(0.97);
  }
}

.coach-name {
  margin-top: 8px;
  max-width: 88px;
  @include md-title-small;
  color: var(--md-on-surface);
}

.coach-sub {
  margin-top: 2px;
  max-width: 88px;
  @include md-label-small;
  color: var(--md-on-surface-variant);
}

// 骨架屏辅助
.skeleton-entries {
  margin-top: 12px;
  padding: 16px 0;
}

.entry-skeleton {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.skeleton-lines {
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
</style>
