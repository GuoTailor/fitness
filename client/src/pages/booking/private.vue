<template>
  <view class="md-page">
    <M3TopAppBar :title="coachName || '预约私教'" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 7 日横滑日期条（明天起） -->
      <view class="days">
        <view
          v-for="d in days"
          :key="d.date"
          class="day"
          :class="{ 'is-active': activeDate === d.date }"
          @click="onDate(d.date)"
        >
          <text class="day__label">{{ d.label }}</text>
          <text class="day__sub">{{ d.sub }}</text>
        </view>
      </view>

      <!-- 选择时间 -->
      <M3Card variant="outlined" class="section-card">
        <view class="section-card__title">选择时间（每节课 60 分钟）</view>
        <view v-if="slotsLoading" class="slot-grid">
          <M3Skeleton v-for="i in 6" :key="i" variant="rect" width="31%" height="36px" />
        </view>
        <template v-else>
          <view v-if="slots.length" class="slot-grid">
            <view
              v-for="(s, i) in slots"
              :key="s.startMinute"
              class="slot"
              :class="{ 'is-active': selectedSlot === i }"
              @click="selectedSlot = i"
            >
              <text>{{ s.label }}</text>
            </view>
          </view>
          <text v-else class="inline-empty">当天暂无可约时段</text>
        </template>
      </M3Card>

      <!-- 选择课包 -->
      <M3Card variant="outlined" class="section-card">
        <view class="section-card__title">选择课包</view>
        <template v-if="mps.length">
          <view class="mp-pick" @click="sheetOpen = true">
            <view class="mp-pick__info">
              <text class="mp-pick__title md-ellipsis">
                {{ selectedMp >= 0 ? mps[selectedMp].packageTitle : '请选择课包' }}
              </text>
              <text v-if="selectedMp >= 0" class="mp-pick__sub">
                剩余 {{ mps[selectedMp].totalSessions - mps[selectedMp].usedSessions }} 节
              </text>
            </view>
            <M3Button variant="tonal" size="sm" @click="sheetOpen = true">选择</M3Button>
          </view>
        </template>
        <template v-else>
          <text class="inline-empty">暂无可用课包，请先购买</text>
          <view class="mp-buy">
            <M3Button variant="text" @click="goCourseList">去购课</M3Button>
          </view>
        </template>
      </M3Card>

      <!-- 底部占位 -->
      <view class="footer-space" />
    </view>

    <!-- 固定提交栏 -->
    <view class="footer-bar">
      <view class="md-shell footer-bar__inner">
        <M3Button block size="lg" :loading="submitting" @click="onSubmit">
          提交预约（扣 1 课时）
        </M3Button>
      </view>
    </view>

    <!-- 课包选择弹层 -->
    <M3BottomSheet :open="sheetOpen" title="选择课包" @close="sheetOpen = false">
      <view
        v-for="(m, i) in mps"
        :key="m.id"
        class="mp-option"
        :class="{ 'is-active': selectedMp === i }"
        @click="onPickMp(i)"
      >
        <view class="mp-option__info">
          <text class="mp-option__title md-ellipsis">{{ m.packageTitle }}</text>
          <text class="mp-option__sub">剩余 {{ m.totalSessions - m.usedSessions }} 节</text>
        </view>
        <view class="mp-option__check">
          <M3Icon v-if="selectedMp === i" name="check_circle" :size="22" />
        </view>
      </view>
    </M3BottomSheet>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onPageScroll } from '@dcloudio/uni-app'
import { useAuthStore } from '@/stores/auth'
import { request } from '@/utils/request'

const auth = useAuthStore()

const coachId = ref(0)
const coachName = ref('')
const days = ref([])
const activeDate = ref('')
const slots = ref([])
const slotsLoading = ref(true)
const selectedSlot = ref(-1)
const mps = ref([]) // 可用课包（该教练在售）
const selectedMp = ref(-1)
const sheetOpen = ref(false)
const submitting = ref(false)
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

const WEEK = ['日', '一', '二', '三', '四', '五', '六']
const addDays = (d, n) => new Date(d.getTime() + n * 86400000)
const iso = (d) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`

onLoad(async (query) => {
  const list = []
  for (let i = 1; i <= 7; i++) {
    const d = addDays(new Date(), i)
    list.push({
      date: iso(d),
      label: `周${WEEK[d.getDay()]}`,
      sub: `${d.getMonth() + 1}/${d.getDate()}`
    })
  }
  coachId.value = Number((query && query.coachId) || 0)
  coachName.value = query && query.coachName ? decodeURIComponent(query.coachName) : ''
  days.value = list
  activeDate.value = list[0].date

  await auth.ensureLogin().catch(() => {})
  loadSlots(activeDate.value)
  loadPackages()
})

async function loadSlots(date) {
  slotsLoading.value = true
  try {
    const data = await request({ url: `/api/v1/coaches/${coachId.value}/slots?date=${date}` })
    slots.value = (data && data.slots) || []
  } catch (e) {
    slots.value = []
  } finally {
    slotsLoading.value = false
  }
}

function onDate(date) {
  if (activeDate.value === date) return
  activeDate.value = date
  selectedSlot.value = -1
  loadSlots(date)
}

async function loadPackages() {
  try {
    const list = await request({ url: '/api/v1/member/packages' })
    const usable = (list || []).filter((p) => p.status === 'active' || p.status === 'inactive')
    mps.value = usable
    selectedMp.value = usable.length ? 0 : -1
  } catch (e) {
    mps.value = []
  }
}

function onPickMp(i) {
  selectedMp.value = i
  sheetOpen.value = false
}

async function onSubmit() {
  if (submitting.value) return
  if (selectedSlot.value < 0) {
    uni.showToast({ title: '请选择时间', icon: 'none' })
    return
  }
  if (selectedMp.value < 0) {
    uni.showToast({ title: '暂无可用课包，请先购买', icon: 'none' })
    return
  }
  submitting.value = true
  try {
    await request({
      url: '/api/v1/bookings',
      method: 'POST',
      data: {
        coachId: coachId.value,
        memberPackageId: mps.value[selectedMp.value].id,
        date: activeDate.value,
        startMinute: slots.value[selectedSlot.value].startMinute
      }
    })
    // 授权订阅：预约与确认通知（仅小程序端）
    // #ifdef MP-WEIXIN
    subscribeScenes(['booking_new', 'booking_confirmed'])
    // #endif
    uni.showModal({
      title: '预约成功',
      content: '已扣 1 课时，等待教练确认',
      confirmText: '看预约',
      cancelText: '返回',
      success: (r) => {
        if (r.confirm) uni.navigateTo({ url: '/pages/booking/list' })
      }
    })
    loadPackages()
  } catch (e) {
    // toast 已由请求层统一处理
  } finally {
    submitting.value = false
  }
}

function goCourseList() {
  uni.reLaunch({ url: '/pages/course/list' })
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
.days {
  display: flex;
  gap: 8px;
  margin-bottom: 4px;
}
.day {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 10px 0;
  border-radius: $md-shape-md;
  background: var(--md-surface-container-high);
  cursor: pointer;
  transition: background $md-motion-short, color $md-motion-short;
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

// 分区卡片
.section-card {
  margin-top: 12px;
  padding: 16px;
  &__title {
    @include md-title-medium;
    color: var(--md-on-surface);
    margin-bottom: 12px;
  }
}

// 时段网格
.slot-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.slot {
  width: 31%;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--md-outline-variant);
  border-radius: $md-shape-sm;
  color: var(--md-on-surface-variant);
  @include md-body-medium;
  cursor: pointer;
  transition: background $md-motion-short, border-color $md-motion-short, color $md-motion-short;
  &:active {
    background: var(--md-surface-container-high);
  }
  &.is-active {
    border: 2px solid var(--md-primary);
    background: var(--md-primary-container);
    color: var(--md-on-primary-container);
    font-weight: 600;
  }
}

// 课包选择入口
.mp-pick {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 4px 0;
  cursor: pointer;
  &:active {
    opacity: 0.8;
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
}
.mp-buy {
  display: flex;
  justify-content: center;
}

// 弹层内课包选项
.mp-option {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 4px;
  border-radius: $md-shape-md;
  cursor: pointer;
  &.is-active .mp-option__check {
    color: var(--md-primary);
  }
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
  &__check {
    color: var(--md-outline-variant);
    flex-shrink: 0;
  }
}

.inline-empty {
  display: block;
  text-align: center;
  padding: 16px 0;
  @include md-body-small;
  color: var(--md-outline);
}

// 固定提交栏
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
