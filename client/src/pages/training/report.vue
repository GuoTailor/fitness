<template>
  <view class="md-page">
    <M3TopAppBar title="训练周报" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 周切换 -->
      <view class="week-nav">
        <M3IconButton icon="arrow_back" :size="20" @click="onPrev" />
        <text class="week-text md-ellipsis">{{ weekText }}</text>
        <M3IconButton icon="arrow_forward" :size="20" @click="onNext" />
      </view>

      <!-- 骨架 -->
      <template v-if="loading && !report">
        <view class="stat-grid">
          <view v-for="i in 4" :key="i" class="stat-cell">
            <M3Skeleton variant="text" width="48" :height="28" />
            <M3Skeleton variant="text" width="72" :height="12" />
          </view>
        </view>
        <M3Card variant="filled" class="block-card">
          <M3Skeleton variant="text" width="120" :height="16" />
          <M3Skeleton variant="rect" :height="56" />
        </M3Card>
      </template>

      <template v-else-if="report">
        <!-- 统计卡 -->
        <view class="stat-grid">
          <view class="stat-cell">
            <text class="stat-v">{{ report.current.sessions }}</text>
            <text class="stat-l">上课（{{ report.deltas.sessions }}）</text>
          </view>
          <view class="stat-cell">
            <text class="stat-v">{{ report.current.minutes }}</text>
            <text class="stat-l">分钟（{{ report.deltas.minutes }}）</text>
          </view>
          <view class="stat-cell">
            <text class="stat-v">{{ report.current.activeDays }}</text>
            <text class="stat-l">训练天数（{{ report.deltas.activeDays }}）</text>
          </view>
          <view class="stat-cell">
            <text class="stat-v">{{ report.weight.deltaText || '—' }}</text>
            <text class="stat-l">体重变化</text>
          </view>
        </view>

        <!-- 课程构成 -->
        <M3Card variant="filled" class="block-card">
          <view class="card-title-row">
            <M3Icon name="bar_chart" :size="20" class="card-title-icon" />
            <text class="sec-title">课程构成</text>
          </view>
          <template v-if="report.current.sessions">
            <view class="src-row">
              <view class="src-chip">
                <M3Icon name="fitness_center" :size="16" class="src-icon" />
                <text>私教 {{ report.current.private }} 节</text>
              </view>
              <view class="src-chip">
                <M3Icon name="groups" :size="16" class="src-icon" />
                <text>团课 {{ report.current.group }} 节</text>
              </view>
            </view>
          </template>
          <text v-else class="block-empty">本周还没有训练记录</text>
        </M3Card>

        <!-- 教练课时榜 -->
        <M3Card v-if="report.byCoach.length" variant="filled" class="block-card">
          <view class="card-title-row">
            <M3Icon name="emoji_events" :size="20" class="card-title-icon" />
            <text class="sec-title">教练课时榜</text>
          </view>
          <view v-for="(c, i) in report.byCoach" :key="c.coachId" class="coach-row">
            <text class="coach-rank" :class="{ 'is-top': i === 0 }">{{ i + 1 }}</text>
            <text class="coach-name md-ellipsis">{{ c.coachName || '教练 #' + c.coachId }}</text>
            <text class="coach-num">{{ c.sessions }} 节 · {{ c.minutes }} 分钟</text>
          </view>
        </M3Card>

        <!-- 本周体重 -->
        <M3Card v-if="report.weight.first" variant="filled" class="block-card">
          <view class="card-title-row">
            <M3Icon name="monitor_weight" :size="20" class="card-title-icon" />
            <text class="sec-title">本周体重</text>
          </view>
          <view class="w-line">
            <view class="w-point">
              <text class="w-label">周一</text>
              <text class="w-val">{{ kg(report.weight.first) }}kg</text>
            </view>
            <M3Icon name="trending_up" :size="20" class="w-arrow" />
            <view class="w-point">
              <text class="w-label">最近</text>
              <text class="w-val">{{ kg(report.weight.last) }}kg</text>
            </view>
          </view>
        </M3Card>

        <!-- 上周对比 -->
        <view class="prev-tip">
          <M3Icon name="history" :size="14" class="prev-tip-icon" />
          <text>上周对比：上课 {{ report.prev.sessions }} 节 · {{ report.prev.minutes }} 分钟</text>
        </view>
      </template>
    </view>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onPageScroll } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()

const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

const date = ref('')
const weekText = ref('')
const report = ref(null)
const loading = ref(true)

function isoDate(d) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function kg(weightG) {
  return (Number(weightG || 0) / 1000).toFixed(1)
}

onLoad(async () => {
  date.value = isoDate(new Date())
  await auth.ensureLogin().catch(() => {})
  load()
})

function shiftWeek(days) {
  const d = new Date(date.value + 'T00:00:00')
  d.setDate(d.getDate() + days)
  date.value = isoDate(d)
  load()
}

function onPrev() { shiftWeek(-7) }
function onNext() { shiftWeek(7) }

function delta(cur, prev, key) {
  const c = cur[key] || 0
  const p = prev[key] || 0
  if (p === 0) return c > 0 ? '新纪录' : '—'
  const pct = Math.round(((c - p) / p) * 100)
  return (pct > 0 ? '+' : '') + pct + '%'
}

async function load() {
  loading.value = true
  try {
    const rep = await request({ url: '/api/v1/training/report', data: { date: date.value } })
    rep.current = rep.current || {}
    rep.prev = rep.prev || {}
    rep.byCoach = rep.byCoach || []
    rep.weight = rep.weight || {}
    rep.deltas = {
      sessions: delta(rep.current, rep.prev, 'sessions'),
      minutes: delta(rep.current, rep.prev, 'minutes'),
      activeDays: delta(rep.current, rep.prev, 'activeDays')
    }
    if (rep.weight.deltaKg !== null && rep.weight.deltaKg !== undefined) {
      const dkg = rep.weight.deltaKg
      rep.weight.deltaText = (dkg > 0 ? '+' : '') + dkg.toFixed(1) + 'kg'
    }
    report.value = rep
    weekText.value = `${rep.weekStart} ~ ${rep.weekEnd}`
  } catch (e) {
    // 错误已由请求层 toast
  } finally {
    loading.value = false
  }
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

// ---- 周切换 ----
.week-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 14px;
}

.week-text {
  @include md-title-medium;
  color: var(--md-on-surface);
}

// ---- 统计卡 ----
.stat-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.stat-cell {
  width: calc(50% - 4px);
  padding: 16px;
  border-radius: $md-shape-md;
  background: var(--md-surface-container-high);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.stat-v {
  @include md-headline-small;
  font-weight: 600;
  color: var(--md-primary);
}

.stat-l {
  @include md-label-medium;
  color: var(--md-on-surface-variant);
  text-align: center;
}

// ---- 卡片 ----
.block-card {
  padding: 16px;
  margin-bottom: 12px;
}

.sec-title {
  @include md-title-medium;
  color: var(--md-on-surface);
}

.card-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}

.card-title-icon {
  color: var(--md-primary);
}

.block-empty {
  display: block;
  padding: 4px 0 2px;
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

// ---- 课程构成 ----
.src-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.src-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 14px;
  border-radius: $md-shape-sm;
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);
  @include md-label-large;
}

.src-icon {
  flex-shrink: 0;
}

// ---- 教练课时榜 ----
.coach-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 0;

  & + .coach-row {
    border-top: 1px solid var(--md-outline-variant);
  }
}

.coach-rank {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface-variant);
  display: flex;
  align-items: center;
  justify-content: center;
  @include md-label-medium;
  flex-shrink: 0;

  &.is-top {
    background: var(--md-primary-container);
    color: var(--md-on-primary-container);
    font-weight: 600;
  }
}

.coach-name {
  flex: 1;
  min-width: 0;
  @include md-body-large;
  color: var(--md-on-surface);
}

.coach-num {
  @include md-label-medium;
  color: var(--md-on-surface-variant);
  flex-shrink: 0;
}

// ---- 本周体重 ----
.w-line {
  display: flex;
  align-items: center;
  justify-content: space-around;
  gap: 12px;
  padding: 4px 0;
}

.w-point {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.w-label {
  @include md-label-medium;
  color: var(--md-on-surface-variant);
}

.w-val {
  @include md-title-large;
  font-weight: 600;
  color: var(--md-primary);
}

.w-arrow {
  color: var(--md-on-surface-variant);
  flex-shrink: 0;
}

// ---- 上周对比 ----
.prev-tip {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 4px 0 8px;
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

.prev-tip-icon {
  color: var(--md-on-surface-variant);
}
</style>
