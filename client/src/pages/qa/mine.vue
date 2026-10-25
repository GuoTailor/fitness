<template>
  <view class="md-page">
    <M3TopAppBar title="我的提问" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 首屏骨架 -->
      <template v-if="firstLoading">
        <M3Card v-for="i in 2" :key="i" variant="filled" class="q-card q-skeleton">
          <M3Skeleton variant="text" width="160" :height="16" />
          <M3Skeleton variant="rect" :height="40" />
        </M3Card>
      </template>

      <!-- 空态 -->
      <M3EmptyState
        v-else-if="!list.length"
        icon="question_answer"
        title="还没有提问"
        description="在课包详情页可以向教练提问"
      />

      <!-- 问答列表 -->
      <template v-else>
        <M3Card v-for="q in list" :key="q.id" variant="filled" class="q-card">
          <view class="q-head">
            <text class="q-pkg md-ellipsis">{{ q.packageTitle || '课包提问' }}</text>
            <view class="q-status" :class="{ 'is-hidden': q.status === 'hidden' }">
              <view class="q-status-dot" />
              <text>{{ q.status === 'hidden' ? '已隐藏' : '已展示' }}</text>
            </view>
          </view>

          <text class="q-content">{{ q.content }}</text>
          <text class="q-meta">{{ q.timeText }}</text>

          <template v-if="q.answers && q.answers.length">
            <M3Divider class="q-divider" />
            <view
              v-for="a in q.answers"
              :key="a.id"
              class="a-item"
              :class="{ 'is-coach': a.isCoach }"
            >
              <view class="a-head">
                <text class="a-badge">{{ a.isCoach ? '教练' : '学员' }}</text>
                <view v-if="a.pinned" class="a-pin">置顶</view>
                <text class="a-author md-ellipsis">{{ a.authorName || (a.isCoach ? '教练' : '学员') }}</text>
              </view>
              <text class="a-content">{{ a.content }}</text>
              <text class="a-meta">{{ a.timeText }}</text>
            </view>
          </template>
          <view v-else class="a-wait">
            <M3Icon name="schedule" :size="14" class="a-wait-icon" />
            <text>等待教练回复</text>
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

const list = ref([])
const firstLoading = ref(true)

onLoad(async () => {
  await auth.ensureLogin().catch(() => {})
})

onShow(() => {
  if (!auth.token) return
  load()
})

async function load() {
  try {
    const rows = await request({ url: '/api/v1/qa/mine' })
    list.value = (rows || []).map((q) => ({
      ...q,
      timeText: fmtTime(q.createdAt),
      answers: (q.answers || []).map((a) => ({ ...a, timeText: fmtTime(a.createdAt) }))
    }))
  } catch (e) {
    // 错误已由请求层 toast
  } finally {
    firstLoading.value = false
  }
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

.q-card {
  padding: 16px;
  margin-bottom: 12px;
}

.q-skeleton {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.q-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.q-pkg {
  flex: 1;
  min-width: 0;
  @include md-title-medium;
  color: var(--md-primary);
}

.q-status {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  @include md-label-small;
  color: var(--md-on-surface-variant);

  &-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--md-primary);
  }

  &.is-hidden {
    color: var(--md-error);
    .q-status-dot {
      background: var(--md-error);
    }
  }
}

.q-content {
  display: block;
  margin-top: 10px;
  @include md-body-medium;
  color: var(--md-on-surface);
  line-height: 22px;
}

.q-meta {
  display: block;
  margin-top: 8px;
  @include md-label-small;
  color: var(--md-on-surface-variant);
}

.q-divider {
  margin: 12px 0;
}

// ---- 回复块 ----
.a-item {
  padding: 10px 12px;
  border-radius: $md-shape-sm;
  background: var(--md-surface-container-lowest);

  & + .a-item {
    margin-top: 8px;
  }

  &.is-coach {
    background: var(--md-primary-container);
  }
}

.a-head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.a-badge {
  padding: 1px 8px;
  border-radius: $md-shape-xs;
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);
  @include md-label-small;
  flex-shrink: 0;
}

.is-coach .a-badge {
  background: var(--md-primary);
  color: var(--md-on-primary);
}

.a-pin {
  padding: 1px 8px;
  border-radius: $md-shape-xs;
  background: var(--md-tertiary-container);
  color: var(--md-on-tertiary-container);
  @include md-label-small;
  flex-shrink: 0;
}

.a-author {
  flex: 1;
  min-width: 0;
  @include md-label-medium;
  color: var(--md-on-surface-variant);
  text-align: right;
}

.a-content {
  display: block;
  margin-top: 8px;
  @include md-body-medium;
  color: var(--md-on-surface);
  line-height: 21px;
}

.a-meta {
  display: block;
  margin-top: 6px;
  @include md-label-small;
  color: var(--md-on-surface-variant);
}

.is-coach .a-meta,
.is-coach .a-author {
  color: var(--md-on-primary-container);
  opacity: 0.72;
}

// ---- 等待回复 ----
.a-wait {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  padding: 10px 12px;
  border-radius: $md-shape-sm;
  background: var(--md-surface-container-lowest);
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

.a-wait-icon {
  color: var(--md-on-surface-variant);
}
</style>
