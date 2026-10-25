<template>
  <view class="md-page">
    <M3TopAppBar title="教练工作台" :back="true" :elevated="scrolled" />

    <!-- 身份门槛 -->
    <view v-if="!checked" class="md-shell page-body">
      <view class="sk-card">
        <M3Skeleton variant="text" width="40%" height="18" />
        <M3Skeleton variant="text" width="70%" height="14" />
      </view>
      <M3Skeleton variant="rect" height="84px" class="sk-gap" />
      <M3Skeleton variant="rect" height="120px" class="sk-gap" />
    </view>

    <M3EmptyState
      v-else-if="!auth.isCoach"
      icon="manage_accounts"
      title="需要教练身份"
      description="登录教练账号后即可管理预约、课程与学员"
    >
      <M3Button variant="filled" icon="how_to_reg" @click="goApply">去入驻</M3Button>
    </M3EmptyState>

    <view v-else class="md-shell page-body">
      <!-- 日期切换条（默认今天） -->
      <scroll-view scroll-x class="day-strip" :show-scrollbar="false">
        <view class="day-strip__inner">
          <view
            v-for="d in days"
            :key="d.date"
            class="day"
            :class="{ 'is-active': date === d.date }"
            @click="onDay(d.date)"
          >
            <text class="day__label">{{ d.label }}</text>
            <text class="day__sub">{{ d.sub }}</text>
          </view>
        </view>
      </scroll-view>

      <!-- 业绩统计卡 -->
      <M3Card variant="filled" class="stats-card">
        <view v-if="!stats" class="stats-row">
          <view v-for="i in 3" :key="i" class="stat">
            <M3Skeleton variant="text" width="48" height="26" />
            <M3Skeleton variant="text" width="60" height="12" />
          </view>
        </view>
        <view v-else class="stats-row">
          <view class="stat">
            <text class="stat__v">{{ stats.monthLessons }}</text>
            <text class="stat__l">本月课时</text>
          </view>
          <view class="stat">
            <text class="stat__v">{{ stats.monthStudents }}</text>
            <text class="stat__l">本月学员</text>
          </view>
          <view class="stat">
            <text class="stat__v">{{ stats.ratingAvg || '—' }}</text>
            <text class="stat__l">评分({{ stats.ratingCount }})</text>
          </view>
        </view>
      </M3Card>

      <!-- 管理入口宫格 -->
      <M3Card variant="filled" class="entries">
        <view class="entries-grid">
          <view v-for="e in ENTRIES" :key="e.key" class="entry" @click="go(e.url)">
            <view class="entry-icon" :class="e.tone"><M3Icon :name="e.icon" :size="24" /></view>
            <text class="entry-label">{{ e.label }}</text>
          </view>
        </view>
      </M3Card>

      <!-- 预约列表 -->
      <M3Card variant="filled" class="bk-card">
        <view class="bk-head">
          <text class="bk-title">预约列表</text>
          <text class="bk-date">{{ date }}</text>
        </view>

        <!-- 骨架 -->
        <template v-if="loading && !loadedOnce">
          <view v-for="i in 3" :key="i" class="bk-row">
            <view class="bk-main">
              <M3Skeleton variant="text" width="120px" height="16" />
              <M3Skeleton variant="text" width="64px" height="12" />
            </view>
          </view>
        </template>

        <template v-else>
          <view v-for="r in dayList" :key="r.booking.id" class="bk-row">
            <view class="bk-main">
              <text class="bk-time">{{ r.startLabel }}</text>
              <text class="bk-name md-ellipsis">{{ r.userName || '学员' }}</text>
            </view>
            <view class="st" :style="{ color: bkStatus(r.booking.status)[1] }">
              <view class="st__dot" :style="{ background: bkStatus(r.booking.status)[1] }" />
              <text class="st__label">{{ bkStatus(r.booking.status)[0] }}</text>
            </view>
          </view>
          <!-- 待确认操作 -->
          <view v-for="r in dayPending" :key="'op-' + r.booking.id" class="bk-ops">
            <text class="bk-ops__time">{{ r.startLabel }} · {{ r.userName || '学员' }}</text>
            <view class="bk-ops__btns">
              <M3Button
                variant="filled"
                size="sm"
                icon="check"
                :loading="actingId === r.booking.id"
                @click="onConfirm(r)"
              >确认</M3Button>
              <M3Button variant="outlined" size="sm" @click="askReject(r)">拒绝</M3Button>
            </view>
          </view>

          <view v-if="!loading && !dayList.length" class="bk-empty">
            <M3Icon name="event_available" :size="36" class="bk-empty__icon" />
            <text class="bk-empty__text">{{ date === todayStr() ? '今天暂无私教预约' : '该日期暂无私教预约' }}</text>
          </view>
        </template>
      </M3Card>
    </view>

    <!-- 拒绝预约确认 -->
    <M3Dialog :open="!!rejectRow" title="拒绝预约" icon="help" :dismissible="false">
      <text class="dialog-text">拒绝后将退还学员课时并释放时段，确定拒绝「{{ rejectRow ? rejectRow.userName || '学员' : '' }}」的预约吗？</text>
      <template #actions>
        <M3Button variant="text" @click="rejectRow = null">取消</M3Button>
        <M3Button variant="filled" danger :loading="!!rejectRow && actingId === rejectRow.booking.id" @click="doReject">拒绝</M3Button>
      </template>
    </M3Dialog>
  </view>
</template>

<script setup>
import { computed, ref } from 'vue'
import { onLoad, onShow, onPageScroll } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()

const checked = ref(false) // 身份检查完成
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

// ---- 日期切换条（14 天，默认今天） ----
const WEEK = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
const iso = (d) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
const todayStr = () => iso(new Date())

const days = ref([])
const date = ref('')

// ---- 数据 ----
const stats = ref(null)
const dayList = ref([])
const loading = ref(false)
const loadedOnce = ref(false)

// 预约状态 → 文案 + MD3 语义色
const BK_STATUS = {
  pending: ['待确认', 'var(--md-tertiary)'],
  confirmed: ['已确认', 'var(--md-primary)'],
  completed: ['已完成', 'var(--md-on-surface-variant)']
}
const bkStatus = (s) => BK_STATUS[s] || [s, 'var(--md-on-surface-variant)']

// 管理入口宫格（primary-container 图标容器）
const ENTRIES = [
  { key: 'verify', icon: 'qr_code_scanner', label: '扫码核销', tone: 'is-primary', url: '/pages/coach-admin/verify' },
  { key: 'schedule', icon: 'event_available', label: '可约时段', tone: 'is-secondary', url: '/pages/coach-admin/schedule' },
  { key: 'groups', icon: 'groups', label: '团课管理', tone: 'is-tertiary', url: '/pages/coach-admin/groups' },
  { key: 'reviews', icon: 'reviews', label: '学员评价', tone: 'is-secondary', url: '/pages/coach-admin/reviews' },
  { key: 'feedback', icon: 'chat_bubble', label: '学员反馈', tone: 'is-tertiary', url: '/pages/coach-admin/feedback' },
  { key: 'qa', icon: 'forum', label: '留言答疑', tone: 'is-primary', url: '/pages/coach-admin/qa' }
]

// 待确认的预约（需要操作按钮）
const dayPending = computed(() => dayList.value.filter((r) => r.booking.status === 'pending'))

const actingId = ref(0)
const rejectRow = ref(null)

onLoad(async () => {
  // 登录门槛：MP 静默登录；H5 未登录跳登录页（throw 已 catch）
  await auth.ensureLogin().catch(() => {})
  if (auth.token && !auth.isCoach) await auth.refreshMe()
  checked.value = true
  if (!auth.isCoach) return

  const list = []
  for (let i = 0; i < 14; i++) {
    const d = new Date(Date.now() + i * 86400000)
    list.push({
      date: iso(d),
      label: i === 0 ? '今天' : WEEK[d.getDay()],
      sub: `${d.getMonth() + 1}/${d.getDate()}`
    })
  }
  days.value = list
  date.value = list[0].date
  load()
})

// 从子页返回时刷新（确认/核销后数据可能变化）
onShow(() => {
  if (checked.value && auth.isCoach && date.value) load()
})

function onDay(d) {
  if (date.value === d) return
  date.value = d
  load()
}

async function load() {
  loading.value = true
  try {
    const [day, st] = await Promise.all([
      request({ url: `/api/v1/coach/bookings?date=${date.value}` }),
      request({ url: '/api/v1/coach/stats', silent: true }).catch(() => null)
    ])
    dayList.value = day || []
    if (st) stats.value = st
    loadedOnce.value = true
  } catch (e) {
    // toast 已由请求层处理
  } finally {
    loading.value = false
  }
}

function go(url) {
  uni.navigateTo({ url })
}

function goApply() {
  uni.navigateTo({ url: '/pages/coach/apply' })
}

// 确认预约
async function onConfirm(r) {
  if (actingId.value) return
  actingId.value = r.booking.id
  try {
    await request({ url: `/api/v1/coach/bookings/${r.booking.id}/confirm`, method: 'POST' })
    uni.showToast({ title: '已确认', icon: 'success' })
    load()
  } catch (e) {
    // toast 已由请求层处理
  } finally {
    actingId.value = 0
  }
}

function askReject(r) {
  rejectRow.value = r
}

async function doReject() {
  const r = rejectRow.value
  if (!r || actingId.value) return
  actingId.value = r.booking.id
  try {
    await request({ url: `/api/v1/coach/bookings/${r.booking.id}/reject`, method: 'POST' })
    rejectRow.value = null
    uni.showToast({ title: '已拒绝', icon: 'none' })
    load()
  } catch (e) {
    // toast 已由请求层处理
  } finally {
    actingId.value = 0
  }
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

// 骨架
.sk-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px;
  border-radius: $md-shape-md;
  background: var(--md-surface-container-high);
  margin-bottom: 12px;
}
.sk-gap {
  margin-bottom: 12px;
}

// 日期切换条
.day-strip {
  white-space: nowrap;
  margin-bottom: 4px;
  &__inner {
    display: inline-flex;
    gap: 8px;
    padding: 4px 0 8px;
  }
}
.day {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  width: 64px;
  padding: 10px 0;
  border-radius: $md-shape-md;
  background: var(--md-surface-container-high);
  cursor: pointer;
  flex-shrink: 0;
  transition: background $md-motion-short;
  &:active {
    opacity: 0.85;
  }
  &.is-active {
    background: var(--md-primary);
    .day__label,
    .day__sub {
      color: var(--md-on-primary);
    }
  }
  &__label {
    @include md-title-small;
    color: var(--md-on-surface);
  }
  &__sub {
    margin-top: 2px;
    @include md-label-small;
    color: var(--md-on-surface-variant);
  }
}

// 统计卡
.stats-card {
  margin-bottom: 12px;
}
.stats-row {
  display: flex;
  padding: 16px 0;
}
.stat {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  &__v {
    @include md-headline-small;
    font-weight: 700;
    color: var(--md-on-surface);
  }
  &__l {
    @include md-label-small;
    color: var(--md-on-surface-variant);
  }
}

// 入口宫格
.entries {
  margin-bottom: 12px;
  padding: 16px 0;
}
.entries-grid {
  display: flex;
  flex-wrap: wrap;
}
.entry {
  width: 25%;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 8px 0;
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

// 预约卡
.bk-card {
  padding: 14px 16px 8px;
}
.bk-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 4px;
}
.bk-title {
  @include md-title-medium;
  color: var(--md-on-surface);
  font-weight: 600;
}
.bk-date {
  @include md-label-small;
  color: var(--md-on-surface-variant);
}
.bk-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid var(--md-outline-variant);
}
.bk-main {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: baseline;
  gap: 10px;
}
.bk-time {
  @include md-title-medium;
  font-weight: 700;
  color: var(--md-primary);
  flex-shrink: 0;
}
.bk-name {
  @include md-body-medium;
  color: var(--md-on-surface);
}

// 待确认操作行
.bk-ops {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--md-outline-variant);
  &__time {
    @include md-body-small;
    color: var(--md-on-surface-variant);
  }
  &__btns {
    display: flex;
    gap: 8px;
    flex-shrink: 0;
  }
}

// 空态（卡内）
.bk-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 28px 0;
  &__icon {
    color: var(--md-outline);
  }
  &__text {
    @include md-body-medium;
    color: var(--md-on-surface-variant);
  }
}

// 状态：颜色圆点 + label-small
.st {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  &__dot {
    width: 8px;
    height: 8px;
    border-radius: $md-shape-full;
  }
  &__label {
    @include md-label-small;
  }
}

.dialog-text {
  display: block;
  line-height: 20px;
}
</style>
