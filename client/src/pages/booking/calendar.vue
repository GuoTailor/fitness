<template>
  <view class="md-page">
    <M3TopAppBar title="团课课表" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 14 日横滑日期条 -->
      <scroll-view scroll-x class="day-strip">
        <view class="day-strip__inner">
          <view
            v-for="d in days"
            :key="d.date"
            class="day"
            :class="{ 'is-active': activeDate === d.date }"
            @click="onDay(d.date)"
          >
            <text class="day__label">{{ d.label }}</text>
            <text class="day__sub">{{ d.sub }}</text>
          </view>
        </view>
      </scroll-view>

      <!-- 首屏骨架 -->
      <template v-if="firstLoading">
        <view v-for="i in 2" :key="i" class="sk-card">
          <M3Skeleton variant="text" width="30%" />
          <M3Skeleton variant="text" width="70%" />
          <M3Skeleton variant="text" width="50%" />
        </view>
      </template>

      <template v-else>
        <!-- 当日团课列表 -->
        <template v-if="dayRows.length">
          <M3Card v-for="row in dayRows" :key="row.id" variant="filled" class="gc-card">
            <view class="gc-card__row">
              <view class="gc-card__main">
                <text class="gc-card__time">{{ row.timeText }}</text>
                <text class="gc-card__title md-ellipsis">{{ row.class.title }}</text>
                <view class="gc-card__sub-line">
                  <text class="gc-card__sub md-ellipsis">
                    {{ row.coachName }} · {{ row.class.location }} ·
                  </text>
                  <text class="gc-card__left" :class="{ 'is-short': row.left <= 0 }">
                    余 {{ row.left }}/{{ row.class.capacity }}
                  </text>
                </view>
                <view v-if="statusTag(row).label" class="gc-card__status">
                  <view class="gc-card__dot" :class="'is-' + statusTag(row).tone" />
                  <text>{{ statusTag(row).label }}</text>
                </view>
              </view>
              <view class="gc-card__actions">
                <M3Button
                  v-if="row.class.status === 'enrolling' && row.left > 0 && !row.myStatus"
                  variant="filled"
                  size="sm"
                  @click="onEnroll(row)"
                >报名</M3Button>
                <M3Button
                  v-if="row.myStatus === 'enrolled'"
                  variant="filled"
                  size="sm"
                  icon="qr_code"
                  @click="onCode(row)"
                >核销码</M3Button>
                <M3Button
                  v-if="row.myStatus === 'enrolled'"
                  variant="outlined"
                  size="sm"
                  @click="openQuit(row)"
                >退出</M3Button>
                <M3Button
                  v-if="row.waitlisted"
                  variant="tonal"
                  size="sm"
                  @click="onWaitlistCancel(row)"
                >取消候补</M3Button>
                <M3Button
                  v-else-if="(row.class.status === 'enrolling' && row.left <= 0) || row.class.status === 'fulfilled'"
                  variant="outlined"
                  size="sm"
                  @click="onWaitlist(row)"
                >候补</M3Button>
              </view>
            </view>
          </M3Card>
        </template>

        <!-- 空态 -->
        <M3EmptyState
          v-else
          icon="event"
          title="当天暂无团课"
          description="看看其他日期的课程"
        />
      </template>
    </view>

    <!-- 候补选课包弹层（替代原 ActionSheet） -->
    <M3BottomSheet :open="sheetOpen" title="选择课包（候补）" @close="sheetOpen = false">
      <view
        v-for="m in sheetRows"
        :key="m.id"
        class="wl-option"
        @click="doWaitlist(m)"
      >
        <view class="wl-option__info">
          <text class="wl-option__title md-ellipsis">{{ m.packageTitle }}</text>
          <text class="wl-option__sub">余 {{ m.totalSessions - m.usedSessions }} 节</text>
        </view>
        <M3Icon name="chevron_right" :size="20" class="wl-option__go" />
      </view>
    </M3BottomSheet>

    <!-- 退课确认 -->
    <M3Dialog :open="quitOpen" title="退出团课" icon="help" @close="quitOpen = false">
      <text>退出「{{ quitRow ? quitRow.class.title : '' }}」后，已扣课时将返还到课包。</text>
      <template #actions>
        <M3Button variant="text" @click="quitOpen = false">取消</M3Button>
        <M3Button variant="filled" danger :loading="quitting" @click="doQuit">退出</M3Button>
      </template>
    </M3Dialog>
  </view>
</template>

<script setup>
import { computed, ref } from 'vue'
import { onLoad, onShow, onPageScroll } from '@dcloudio/uni-app'
import { useAuthStore } from '@/stores/auth'
import { request } from '@/utils/request'

const auth = useAuthStore()

const days = ref([])
const activeDate = ref('')
const classesByDate = ref({})
const loading = ref(false)
const firstLoading = ref(true)
const mps = ref([]) // 可用课包
const selectedMp = ref(-1)
const sheetOpen = ref(false)
const sheetRows = ref([])
const pendingWaitlistRow = ref(null)
const quitOpen = ref(false)
const quitRow = ref(null)
const quitting = ref(false)
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

const WEEK = ['日', '一', '二', '三', '四', '五', '六']
const iso = (d) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
const fmtM = (m) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`

onLoad(() => {
  const list = []
  for (let i = 0; i < 14; i++) {
    const d = new Date(Date.now() + i * 86400000)
    list.push({
      date: iso(d),
      label: i === 0 ? '今天' : `周${WEEK[d.getDay()]}`,
      sub: `${d.getMonth() + 1}/${d.getDate()}`
    })
  }
  days.value = list
  activeDate.value = list[0].date
})

onShow(() => {
  loadPkgs()
  load()
})

const dayRows = computed(() => classesByDate.value[activeDate.value] || [])

async function load() {
  if (loading.value) return
  loading.value = true
  try {
    const from = days.value[0].date
    const to = days.value[days.value.length - 1].date
    const data = await request({ url: `/api/v1/group-classes?from=${from}&to=${to}` })
    const byDate = {}
    ;(data || []).forEach((row) => {
      row.timeText = fmtM(row.class.startMinute)
      row.left = row.class.capacity - row.class.enrolledCount
      ;(byDate[row.class.classDate] = byDate[row.class.classDate] || []).push(row)
    })
    classesByDate.value = byDate
  } catch (e) {
    // toast 已由请求层统一处理
  } finally {
    loading.value = false
    firstLoading.value = false
  }
}

async function loadPkgs(silent = false) {
  await auth.ensureLogin().catch(() => {})
  try {
    const list = await request({ url: '/api/v1/member/packages', silent })
    const usable = (list || []).filter((p) => p.status === 'active' || p.status === 'inactive')
    mps.value = usable
    selectedMp.value = usable.length ? Math.min(Math.max(selectedMp.value, 0), usable.length - 1) : -1
  } catch (e) {
    if (!silent) mps.value = []
  }
}

function onDay(date) {
  if (activeDate.value === date) return
  activeDate.value = date
}

// 状态标签（MD3 语义色：成功=primary，警示=error，中性=on-surface-variant）
function statusTag(row) {
  if (row.myStatus === 'enrolled') return { label: '已报名', tone: 'primary' }
  if (row.waitlisted) return { label: '候补中', tone: 'neutral' }
  if (row.class.status === 'fulfilled') return { label: '已满员', tone: 'error' }
  return { label: '', tone: '' }
}

// 报名：先静默刷新课包状态再下单
async function onEnroll(row) {
  await loadPkgs(true)
  if (!mps.value.length || selectedMp.value < 0) {
    uni.showToast({ title: '请先购买对应课包', icon: 'none' })
    return
  }
  const pkgId = Number(row.class.packageId)
  const mp = mps.value.find((m) => m.packageId === pkgId) || mps.value[selectedMp.value]
  try {
    await request({
      url: `/api/v1/group-classes/${row.class.id}/enroll`,
      method: 'POST',
      data: { memberPackageId: mp.id }
    })
    // 授权订阅：报名结果通知（仅小程序端）
    // #ifdef MP-WEIXIN
    subscribeScenes(['booking_confirmed'])
    // #endif
    uni.showToast({ title: '报名成功', icon: 'success' })
    load()
    loadPkgs(true)
  } catch (e) {
    // toast 已由请求层统一处理
  }
}

async function openQuit(row) {
  quitRow.value = row
  quitOpen.value = true
}

async function doQuit() {
  if (!quitRow.value || quitting.value) return
  quitting.value = true
  try {
    await request({ url: `/api/v1/group-classes/${quitRow.value.class.id}/quit`, method: 'POST' })
    quitOpen.value = false
    uni.showToast({ title: '已退出，课时已返还', icon: 'none' })
    load()
    loadPkgs(true)
  } catch (e) {
    // toast 已由请求层统一处理
  } finally {
    quitting.value = false
  }
}

// 满员候补：选课包 → 排队（不扣次，有人退出自动递补扣次）
async function onWaitlist(row) {
  await loadPkgs(true)
  const pkgId = Number(row.class.packageId)
  const matched = mps.value.filter(
    (m) => m.packageId === pkgId && m.totalSessions - m.usedSessions >= 1
  )
  if (!matched.length) {
    uni.showToast({ title: '请先购买对应课包', icon: 'none' })
    return
  }
  if (matched.length > 1) {
    pendingWaitlistRow.value = row
    sheetRows.value = matched
    sheetOpen.value = true
    return
  }
  doWaitlist(matched[0])
}

async function doWaitlist(mp) {
  const row = pendingWaitlistRow.value
  sheetOpen.value = false
  if (!row) return
  try {
    await request({
      url: `/api/v1/group-classes/${row.class.id}/waitlist`,
      method: 'POST',
      data: { memberPackageId: mp.id }
    })
    // 授权订阅：候补递补通知（仅小程序端）
    // #ifdef MP-WEIXIN
    subscribeScenes(['waitlist_promoted'])
    // #endif
    uni.showToast({ title: '已加入候补', icon: 'success' })
    load()
  } catch (e) {
    // toast 已由请求层统一处理
  }
}

async function onWaitlistCancel(row) {
  try {
    await request({ url: `/api/v1/group-classes/${row.class.id}/waitlist/cancel`, method: 'POST' })
    uni.showToast({ title: '已取消候补', icon: 'none' })
    load()
  } catch (e) {
    // toast 已由请求层统一处理
  }
}

function onCode(row) {
  uni.navigateTo({ url: `/pages/booking/detail?type=group&id=${row.class.id}` })
}

// #ifdef MP-WEIXIN
// 订阅消息授权：先查后台已配置模板，有对应场景才弹授权（失败不阻断业务）
function subscribeScenes(scenes) {
  request({ url: '/api/v1/notify/templates', silent: true })
    .then((d) => {
      if (!d || d.mock || !d.templates) return
      const ids = scenes.map((s) => d.templates[s]).filter(Boolean)
      if (ids.length) uni.requestSubscribeMessage({ tmplIds: ids.slice(0, 3) })
    })
    .catch(() => {})
}
// #endif
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

// 日期条
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

// 骨架
.sk-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px;
  margin-top: 12px;
  border-radius: $md-shape-md;
  background: var(--md-surface-container-high);
}

// 团课卡片
.gc-card {
  margin-top: 12px;
  &__row {
    display: flex;
    gap: 12px;
    padding: 14px 16px;
  }
  &__main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  &__time {
    @include md-headline-small;
    color: var(--md-primary);
    font-weight: 700;
  }
  &__title {
    @include md-title-medium;
    color: var(--md-on-surface);
  }
  &__sub-line {
    display: flex;
    align-items: center;
    min-width: 0;
  }
  &__sub {
    @include md-body-small;
    color: var(--md-on-surface-variant);
  }
  &__left {
    @include md-body-small;
    color: var(--md-on-surface-variant);
    flex-shrink: 0;
    &.is-short {
      color: var(--md-error);
      font-weight: 600;
    }
  }
  &__status {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    margin-top: 2px;
    @include md-label-small;
    color: var(--md-on-surface-variant);
  }
  &__dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    &.is-primary {
      background: var(--md-primary);
    }
    &.is-neutral {
      background: var(--md-on-surface-variant);
    }
    &.is-error {
      background: var(--md-error);
    }
  }
  &__actions {
    display: flex;
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
    flex-shrink: 0;
    justify-content: center;
  }
}

// 候补课包选项
.wl-option {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 4px;
  border-radius: $md-shape-md;
  cursor: pointer;
  &:active {
    background: rgba(0, 0, 0, $md-state-pressed-opacity);
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
  &__go {
    color: var(--md-on-surface-variant);
    flex-shrink: 0;
  }
}
</style>
