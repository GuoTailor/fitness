<template>
  <view class="md-page">
    <M3TopAppBar title="课包商城">
      <template #actions>
        <M3IconButton icon="refresh" @click="reload" />
      </template>
    </M3TopAppBar>

    <view class="md-shell page-body">
      <!-- 筛选区 -->
      <view class="filter-bar">
        <scroll-view scroll-x class="chip-row">
          <M3Chip
            v-for="c in categories"
            :key="c.value"
            :label="c.label"
            filter
            :selected="activeCategory === c.value"
            @click="onCategory(c.value)"
          />
        </scroll-view>
        <scroll-view scroll-x class="chip-row">
          <M3Chip
            v-for="t in tags"
            :key="t"
            :label="t || '不限目标'"
            :selected="activeTag === t"
            variant="tonal"
            @click="onTag(t)"
          />
        </scroll-view>
        <M3TextField
          v-model="keyword"
          label="搜索"
          placeholder="搜索课包"
          @confirm="onSearch"
        >
          <template #trail>
            <M3IconButton icon="search" @click="onSearch" />
          </template>
        </M3TextField>
      </view>

      <!-- 首屏骨架 -->
      <template v-if="firstLoading">
        <M3Card v-for="i in 3" :key="i" variant="filled" class="item-skeleton">
          <view class="item-row">
            <M3Skeleton variant="rect" width="100" height="76" />
            <view class="item-skeleton-lines">
              <M3Skeleton variant="text" height="14" />
              <M3Skeleton variant="text" width="120" height="12" />
              <M3Skeleton variant="text" width="80" height="14" />
            </view>
          </view>
        </M3Card>
      </template>

      <!-- 空态 -->
      <M3EmptyState
        v-else-if="!list.length"
        icon="shopping_bag"
        title="暂无课包"
        description="换个筛选条件或关键词试试"
      >
        <M3Button variant="tonal" @click="resetFilter">重置筛选</M3Button>
      </M3EmptyState>

      <!-- 列表 -->
      <template v-else>
        <M3Card
          v-for="p in list"
          :key="p.id"
          variant="filled"
          clickable
          class="pkg-item"
          @click="goPkg(p.id)"
        >
          <view class="item-row">
            <image v-if="p.coverUrl" class="item-cover" :src="p.coverUrl" mode="aspectFill" />
            <view v-else class="item-cover item-cover--ph">
              <M3Icon name="fitness_center" :size="28" />
            </view>
            <view class="item-info">
              <view class="item-title md-ellipsis-2">{{ p.title }}</view>
              <view class="item-meta">
                <text v-if="p.tags" class="item-tag">{{ p.tags }}</text>
                <view v-if="p.ratingCount" class="item-rating">
                  <M3Icon name="star" :size="14" class="item-rating-icon" />
                  <text class="item-rating-text">{{ p.ratingAvg }}（{{ p.ratingCount }}）</text>
                </view>
              </view>
              <view class="item-bottom">
                <view class="item-price">
                  <text class="price-num">¥{{ yuan(p.minPrice) }}</text>
                  <text class="price-unit">起</text>
                </view>
                <text class="item-sold">已售 {{ p.soldCount }}</text>
              </view>
            </view>
          </view>
        </M3Card>

        <!-- 分页状态 -->
        <view v-if="loading" class="page-state">
          <M3Progress type="linear" indeterminate />
        </view>
        <view v-else-if="finished" class="page-state">
          <text class="page-state-text">已加载全部 {{ total }} 个课包</text>
        </view>
        <view v-else class="page-state">
          <text class="page-state-text">上拉加载更多</text>
        </view>
      </template>
    </view>

    <M3BottomNavigation :items="TABS" model-value="course" @change="onTab" />
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onReachBottom } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
import { yuan } from '@/utils/format'

const auth = useAuthStore()

const TABS = [
  { key: 'home', icon: 'home', label: '首页' },
  { key: 'course', icon: 'shopping_bag', label: '课包' },
  { key: 'mine', icon: 'person', label: '我的' }
]

const categories = [
  { value: '', label: '全部' },
  { value: 'private', label: '私教 1v1' },
  { value: 'group', label: '团课卡' },
  { value: 'trial', label: '体验课' }
]
const tags = ['', '减脂', '增肌', '塑形', '康复']

const activeCategory = ref('')
const activeTag = ref('')
const keyword = ref('')
const list = ref([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)
const finished = ref(false)
const firstLoading = ref(false)

onLoad(async (query) => {
  // 登录态：MP 静默登录；H5 未登录跳登录页（throw 已 catch）
  await auth.ensureLogin().catch(() => {})
  if (query && query.category) activeCategory.value = query.category
  firstLoading.value = true
  await load()
  firstLoading.value = false
})

// 触底分页（下拉刷新未启用，顶部提供刷新按钮）
onReachBottom(() => {
  if (!finished.value && !loading.value) load()
})

async function load() {
  loading.value = true
  try {
    const params = { page: page.value, page_size: 10 }
    if (activeCategory.value) params.category = activeCategory.value
    if (activeTag.value) params.tag = activeTag.value
    if (keyword.value.trim()) params.keyword = keyword.value.trim()
    const data = await request({ url: '/api/v1/packages', data: params })
    const items = data.list || []
    list.value = page.value === 1 ? items : list.value.concat(items)
    total.value = data.total || 0
    finished.value = list.value.length >= total.value
    page.value += 1
  } catch (e) {
    // 错误已由请求层 toast
  } finally {
    loading.value = false
  }
}

function reload() {
  page.value = 1
  finished.value = false
  load()
}

function onCategory(v) {
  if (activeCategory.value === v) return
  activeCategory.value = v
  reload()
}
function onTag(v) {
  if (activeTag.value === v) return
  activeTag.value = v
  reload()
}
function onSearch() {
  reload()
}
function resetFilter() {
  activeCategory.value = ''
  activeTag.value = ''
  keyword.value = ''
  reload()
}

function goPkg(id) {
  uni.navigateTo({ url: '/pages/course/detail?id=' + id })
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

.filter-bar {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 16px;
}

.chip-row {
  white-space: nowrap;

  .m3-chip {
    margin-right: 8px;
  }
}

.pkg-item {
  margin-bottom: 12px;
}

.item-row {
  display: flex;
  padding: 12px;
  gap: 12px;
}

.item-cover {
  width: 100px;
  height: 76px;
  border-radius: $md-shape-md;
  background: var(--md-surface-container-highest);
  flex-shrink: 0;
}

.item-cover--ph {
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--md-outline);
}

.item-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}

.item-title {
  @include md-title-small;
  color: var(--md-on-surface);
  line-height: 20px;
}

.item-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 2px 0;
}

.item-tag {
  @include md-label-small;
  background: var(--md-primary-container);
  color: var(--md-on-primary-container);
  border-radius: $md-shape-xs;
  padding: 1px 6px;
}

.item-rating {
  display: flex;
  align-items: center;
  gap: 2px;
}

.item-rating-icon {
  color: var(--md-tertiary);
}

.item-rating-text {
  @include md-label-small;
  color: var(--md-on-surface-variant);
}

.item-bottom {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
}

.item-price {
  display: flex;
  align-items: baseline;
  gap: 2px;
  color: var(--md-primary);
}

.price-num {
  @include md-title-medium;
  font-weight: 700;
  color: var(--md-primary);
}

.price-unit {
  @include md-label-small;
  color: var(--md-on-surface-variant);
}

.item-sold {
  @include md-label-small;
  color: var(--md-on-surface-variant);
}

.page-state {
  padding: 12px 0 4px;
}

.page-state-text {
  display: block;
  text-align: center;
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

.item-skeleton {
  margin-bottom: 12px;
}

.item-skeleton-lines {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
</style>
