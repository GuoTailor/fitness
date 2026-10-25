<template>
  <view class="md-page">
    <M3TopAppBar title="教练团队" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 搜索 -->
      <M3TextField
        v-model="keyword"
        label="搜索教练"
        placeholder="姓名 / 专长"
        @confirm="onSearch"
      >
        <template #trail>
          <M3IconButton icon="search" @click="onSearch" />
        </template>
      </M3TextField>

      <!-- 评分档筛选 -->
      <scroll-view scroll-x class="chip-row">
        <view class="chip-row__inner">
          <M3Chip
            v-for="c in ratingChips"
            :key="c.value"
            :label="c.label"
            filter
            :selected="minRating === c.value"
            @click="onRatingChip(c.value)"
          />
        </view>
      </scroll-view>

      <!-- 首屏骨架 -->
      <template v-if="firstLoading">
        <view v-for="i in 3" :key="i" class="sk-card">
          <M3Skeleton variant="circle" :width="56" :height="56" />
          <view class="sk-card__lines">
            <M3Skeleton variant="text" width="40%" />
            <M3Skeleton variant="text" width="70%" />
          </view>
        </view>
      </template>

      <!-- 教练列表 -->
      <template v-else>
        <M3Card
          v-for="item in list"
          :key="item.id"
          variant="filled"
          clickable
          class="coach-card"
          @click="onCoach(item.id)"
        >
          <view class="coach-card__row">
            <M3Avatar :src="item.avatarUrl" :name="item.name" :size="56" />
            <view class="coach-card__info">
              <view class="coach-card__name-line">
                <text class="coach-card__name md-ellipsis">{{ item.name }}</text>
                <view v-if="item.certVerified" class="coach-card__cert">
                  <M3Icon name="verified" :size="12" />
                  <text>已认证</text>
                </view>
              </view>
              <text class="coach-card__sub md-ellipsis">
                {{ item.specialties || '综合训练' }} · {{ item.years }}年教龄
              </text>
              <view v-if="item.ratingCount" class="coach-card__rating">
                <M3Stars :model-value="Number(item.ratingAvg) || 0" :size="14" show-value />
                <text class="coach-card__rating-count">（{{ item.ratingCount }}条）</text>
              </view>
            </view>
            <M3Icon name="chevron_right" :size="20" class="coach-card__go" />
          </view>
        </M3Card>

        <!-- 加载中 / 到底提示 -->
        <view v-if="loading && list.length" class="loading-more">
          <M3Progress type="linear" indeterminate />
        </view>
        <text v-else-if="finished && list.length" class="end-tip">没有更多教练了</text>

        <!-- 空态 -->
        <M3EmptyState
          v-if="!loading && !list.length"
          icon="groups"
          title="暂无教练"
          description="换个关键词或评分条件试试"
        />
      </template>

      <!-- 教练入驻入口 -->
      <view class="apply-entry">
        <M3Button variant="text" icon="person_add" @click="goApply">我是教练，申请入驻</M3Button>
      </view>
    </view>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onPageScroll, onReachBottom } from '@dcloudio/uni-app'
import { useAuthStore } from '@/stores/auth'
import { request } from '@/utils/request'

const auth = useAuthStore()

const keyword = ref('')
const list = ref([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)
const finished = ref(false)
const firstLoading = ref(true)

const ratingChips = [
  { label: '全部', value: 0 },
  { label: '4.0分以上', value: 4 },
  { label: '4.5分以上', value: 4.5 }
]
const minRating = ref(0)

const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

onLoad(async () => {
  // 小程序端静默登录（H5 为公开页，不强制登录）
  // #ifdef MP-WEIXIN
  await auth.ensureLogin().catch(() => {})
  // #endif
  load()
})

async function load() {
  if (loading.value || finished.value) return
  loading.value = true
  try {
    const data = await request({
      url: '/api/v1/coaches',
      data: {
        page: page.value,
        page_size: 10,
        keyword: keyword.value.trim(),
        minRating: minRating.value
      }
    })
    const next = data.list || []
    list.value = page.value === 1 ? next : list.value.concat(next)
    total.value = data.total || 0
    page.value += 1
    finished.value = list.value.length >= total.value
  } catch (e) {
    // 错误 toast 已由请求层统一处理
  } finally {
    loading.value = false
    firstLoading.value = false
  }
}

function reset() {
  page.value = 1
  finished.value = false
  list.value = []
  firstLoading.value = true
}

function onSearch() {
  reset()
  load()
}

function onRatingChip(v) {
  if (minRating.value === v) return
  minRating.value = v
  reset()
  load()
}

onReachBottom(() => {
  if (!firstLoading.value) load()
})

function onCoach(id) {
  uni.navigateTo({ url: '/pages/coach/detail?id=' + id })
}

function goApply() {
  uni.navigateTo({ url: '/pages/coach/apply' })
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

.chip-row {
  margin: 12px 0 4px;
  white-space: nowrap;
  &__inner {
    display: inline-flex;
    gap: 8px;
    padding: 4px 0 8px;
  }
}

.sk-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px;
  margin-top: 12px;
  border-radius: $md-shape-md;
  background: var(--md-surface-container-high);
  &__lines {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
}

.coach-card {
  margin-top: 12px;
  &__row {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 14px 16px;
  }
  &__info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  &__name-line {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }
  &__name {
    @include md-title-medium;
    color: var(--md-on-surface);
    flex-shrink: 0;
    max-width: 60%;
  }
  &__cert {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    padding: 1px 8px;
    border-radius: $md-shape-xs;
    background: var(--md-primary-container);
    color: var(--md-on-primary-container);
    @include md-label-small;
    flex-shrink: 0;
  }
  &__sub {
    @include md-body-small;
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
  &__go {
    color: var(--md-on-surface-variant);
    flex-shrink: 0;
  }
}

.loading-more {
  margin: 16px 0 0;
}

.end-tip {
  display: block;
  text-align: center;
  padding: 16px 0 0;
  color: var(--md-on-surface-variant);
  @include md-body-small;
}

.apply-entry {
  display: flex;
  justify-content: center;
  padding: 16px 0 0;
}
</style>
