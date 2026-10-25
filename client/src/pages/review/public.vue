<template>
  <view class="md-page">
    <M3TopAppBar title="学员评价" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 统计 -->
      <view v-if="total > 0" class="count-row">
        <text class="count-text">共 {{ total }} 条评价</text>
      </view>

      <!-- 参数缺失 -->
      <M3EmptyState
        v-if="badParams"
        icon="error"
        title="参数缺失"
        description="请从课包或教练主页进入查看评价"
      />

      <!-- 首屏骨架 -->
      <template v-else-if="firstLoading">
        <M3Card v-for="i in 3" :key="i" variant="filled" class="rv-card rv-skeleton">
          <view class="rv-sk-head">
            <M3Skeleton variant="circle" :width="40" :height="40" />
            <view class="rv-sk-lines">
              <M3Skeleton variant="text" width="120" :height="14" />
              <M3Skeleton variant="text" width="80" :height="12" />
            </view>
          </view>
          <M3Skeleton variant="rect" :height="44" />
        </M3Card>
      </template>

      <!-- 空态 -->
      <M3EmptyState
        v-else-if="!items.length"
        icon="reviews"
        title="暂无评价"
        description="上过课的学员可以发表评价"
      />

      <!-- 评价列表 -->
      <template v-else>
        <M3Card v-for="r in items" :key="r.review.id" variant="filled" class="rv-card">
          <view class="rv-head">
            <M3Avatar :src="r.avatarUrl" :name="r.nickname" :size="40" />
            <view class="rv-head-info">
              <view class="rv-nick-row">
                <text class="rv-nick md-ellipsis">{{ r.nickname }}</text>
                <view v-if="r.review.featured" class="rv-featured">
                  <M3Icon name="verified" :size="12" />
                  <text>精选</text>
                </view>
              </view>
              <M3Stars :model-value="r.review.rating" :size="14" />
            </view>
          </view>

          <text v-if="r.review.content" class="rv-content">{{ r.review.content }}</text>

          <view v-if="r.images && r.images.length" class="rv-imgs">
            <image
              v-for="img in r.images"
              :key="img"
              class="rv-img"
              :src="img"
              mode="aspectFill"
              @click="preview(img, r.images)"
            />
          </view>

          <view v-if="r.review.coachReply" class="rv-reply">
            <view class="rv-reply-head">
              <M3Icon name="forum" :size="14" class="rv-reply-icon" />
              <text class="rv-reply-label">教练回复</text>
            </view>
            <text class="rv-reply-text">{{ r.review.coachReply }}</text>
          </view>

          <text class="rv-time">{{ r.timeText }}</text>
        </M3Card>

        <!-- 分页状态 -->
        <view v-if="loading" class="page-state">
          <M3Progress type="linear" indeterminate />
        </view>
        <view v-else-if="done" class="page-state">
          <text class="page-state-text">已加载全部 {{ total }} 条评价</text>
        </view>
        <view v-else class="page-state">
          <text class="page-state-text">上拉加载更多</text>
        </view>
      </template>
    </view>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onPageScroll, onReachBottom } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
import { fmtDate } from '@/utils/format'

const auth = useAuthStore()

const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

// ?type=package|coach&id=X
const type = ref('package')
const id = ref(0)
const items = ref([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)
const done = ref(false)
const firstLoading = ref(false)
const badParams = ref(false)

onLoad(async (query) => {
  type.value = (query && query.type) || 'package'
  id.value = Number(query && query.id) || 0
  if (!id.value) {
    badParams.value = true
    return
  }
  await auth.ensureLogin().catch(() => {})
  firstLoading.value = true
  await loadMore()
  firstLoading.value = false
})

onReachBottom(() => {
  if (!done.value && !loading.value) loadMore()
})

async function loadMore() {
  if (loading.value || done.value || !id.value) return
  loading.value = true
  try {
    const d = await request({
      url: `/api/v1/${type.value === 'coach' ? 'coaches' : 'packages'}/${id.value}/reviews`,
      data: { page: page.value, pageSize: 10 }
    })
    const rows = (d.items || []).map((x) => ({
      ...x,
      nickname: (x.nickname || '学员').slice(0, 12),
      timeText: fmtDate(x.review && x.review.createdAt)
    }))
    items.value = items.value.concat(rows)
    total.value = d.total || 0
    page.value += 1
    done.value = items.value.length >= total.value
  } catch (e) {
    // 错误已由请求层 toast
  } finally {
    loading.value = false
  }
}

// 图片预览（H5 / MP 通用）
function preview(current, urls) {
  uni.previewImage({ current, urls })
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

.count-row {
  padding: 0 4px 10px;
}

.count-text {
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

.rv-card {
  padding: 16px;
  margin-bottom: 12px;
}

.rv-skeleton {
  .rv-sk-head {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 12px;
  }
  .rv-sk-lines {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
}

.rv-head {
  display: flex;
  align-items: center;
  gap: 12px;
}

.rv-head-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.rv-nick-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.rv-nick {
  @include md-title-medium;
  color: var(--md-on-surface);
}

.rv-featured {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 1px 8px;
  border-radius: $md-shape-xs;
  background: var(--md-primary-container);
  color: var(--md-on-primary-container);
  @include md-label-small;
  flex-shrink: 0;
}

.rv-content {
  display: block;
  margin-top: 10px;
  @include md-body-medium;
  color: var(--md-on-surface);
  line-height: 22px;
}

.rv-imgs {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}

.rv-img {
  width: 104px;
  height: 104px;
  border-radius: $md-shape-sm;
  background: var(--md-surface-container-highest);
  &:active {
    opacity: 0.85;
  }
}

.rv-reply {
  margin-top: 12px;
  padding: 10px 12px;
  border-radius: $md-shape-sm;
  background: var(--md-surface-container-lowest);
}

.rv-reply-head {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 4px;
}

.rv-reply-icon {
  color: var(--md-primary);
}

.rv-reply-label {
  @include md-label-medium;
  color: var(--md-primary);
  font-weight: 500;
}

.rv-reply-text {
  @include md-body-medium;
  color: var(--md-on-surface-variant);
  line-height: 20px;
}

.rv-time {
  display: block;
  margin-top: 10px;
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
</style>
