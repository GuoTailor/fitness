<template>
  <view class="md-page">
    <M3TopAppBar title="可约时段" :back="true" :elevated="scrolled" />

    <!-- 身份门槛 -->
    <view v-if="!checked" class="md-shell page-body">
      <view v-for="i in 3" :key="i" class="sk-card">
        <M3Skeleton variant="text" width="30%" height="16" />
        <M3Skeleton variant="text" width="70%" height="14" />
      </view>
    </view>

    <M3EmptyState
      v-else-if="!auth.isCoach"
      icon="manage_accounts"
      title="需要教练身份"
      description="登录教练账号后即可管理可约时段"
    >
      <M3Button variant="filled" icon="how_to_reg" @click="goApply">去入驻</M3Button>
    </M3EmptyState>

    <view v-else class="md-shell page-body">
      <!-- 说明 -->
      <view class="tip">
        <M3Icon name="info" :size="18" class="tip__icon" />
        <text class="tip__text">每周可约模板：学员只能在窗口内预约，每节课 60 分钟、30 分钟粒度起约</text>
      </view>

      <!-- 周模板编辑 -->
      <M3Card v-for="d in items" :key="d.weekday" variant="filled" class="day-card">
        <view class="day-head">
          <text class="day-name">{{ d.label }}</text>
          <M3Button variant="text" size="sm" icon="add" @click="addWindow(d.weekday)">时段</M3Button>
        </view>

        <view v-for="(w, idx) in d.windows" :key="idx" class="win">
          <picker mode="time" :value="w.s" @change="(e) => onTime(d.weekday, idx, 's', e.detail.value)">
            <view class="time-chip">{{ w.s }}</view>
          </picker>
          <text class="sep">至</text>
          <picker mode="time" :value="w.e" @change="(e) => onTime(d.weekday, idx, 'e', e.detail.value)">
            <view class="time-chip">{{ w.e }}</view>
          </picker>
          <M3IconButton icon="delete" :size="20" class="win-del" @click="removeWindow(d.weekday, idx)" />
        </view>

        <text v-if="!d.windows.length" class="win-empty">休息（无可约时段）</text>
      </M3Card>

      <!-- 请假管理 -->
      <M3Card variant="filled" class="leave-card">
        <view class="section-title">请假管理</view>
        <text class="tip2">请假时段学员不可预约；已产生的预约请线下协商改期（系统不自动取消）</text>

        <view class="leave-form">
          <picker mode="date" :value="leaveDate" :start="todayStr()" @change="(e) => (leaveDate = e.detail.value)">
            <view class="time-chip">{{ leaveDate }}</view>
          </picker>
          <picker mode="time" :value="leaveStart" @change="(e) => (leaveStart = e.detail.value)">
            <view class="time-chip">{{ leaveStart }}</view>
          </picker>
          <text class="sep">至</text>
          <picker mode="time" :value="leaveEnd" @change="(e) => (leaveEnd = e.detail.value)">
            <view class="time-chip">{{ leaveEnd }}</view>
          </picker>
          <M3Button variant="tonal" size="sm" :loading="addingLeave" @click="onAddLeave">登记</M3Button>
        </view>

        <view v-for="l in leaves" :key="l.id" class="leave-row">
          <text class="leave-text">{{ l.leaveDate }} {{ l.fullDay ? '全天' : l.rangeText }}</text>
          <M3Button variant="text" size="sm" danger @click="askRemoveLeave(l)">撤销</M3Button>
        </view>
        <text v-if="!leaves.length" class="win-empty">暂无请假记录</text>
      </M3Card>
    </view>

    <!-- 底部保存栏 -->
    <view v-if="checked && auth.isCoach" class="footer-bar">
      <M3Button variant="filled" block :loading="saving" @click="onSave">保存模板</M3Button>
    </view>
    <view v-if="checked && auth.isCoach" class="footer-space" />

    <!-- 撤销请假确认 -->
    <M3Dialog :open="!!removeLeave" title="撤销该请假？" icon="help" :dismissible="false">
      <text class="dialog-text">撤销后该时段恢复可约，学员可重新预约。</text>
      <template #actions>
        <M3Button variant="text" @click="removeLeave = null">取消</M3Button>
        <M3Button variant="filled" danger :loading="removing" @click="doRemoveLeave">撤销</M3Button>
      </template>
    </M3Dialog>

    <!-- 请假影响提示 -->
    <M3Dialog :open="leaveAffected > 0" title="已登记请假" icon="warning" :dismissible="false">
      <text class="dialog-text">该时段已有 {{ leaveAffected }} 个预约，请尽快与学员协商改期</text>
      <template #actions>
        <M3Button variant="filled" @click="leaveAffected = 0">知道了</M3Button>
      </template>
    </M3Dialog>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onPageScroll } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()

const checked = ref(false)
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

const WEEK = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
const fmtM = (m) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`
const toMin = (s) => Number(s.split(':')[0]) * 60 + Number(s.split(':')[1])
const iso = (d) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
const todayStr = () => iso(new Date())
const addDays = (dateStr, n) => {
  const d = new Date(dateStr + 'T00:00:00')
  d.setDate(d.getDate() + n)
  return iso(d)
}

// 展示顺序周一~周日（weekday 值 0=周日，与后端一致）
const DAY_ORDER = [1, 2, 3, 4, 5, 6, 0]

const items = ref(DAY_ORDER.map((wd) => ({ weekday: wd, label: WEEK[wd], windows: [] })))
const saving = ref(false)

// 请假
const leaveDate = ref(todayStr())
const leaveStart = ref('09:00')
const leaveEnd = ref('21:00')
const leaves = ref([])
const addingLeave = ref(false)
const removeLeave = ref(null)
const removing = ref(false)
const leaveAffected = ref(0)

onLoad(async () => {
  await auth.ensureLogin().catch(() => {})
  if (auth.token && !auth.isCoach) await auth.refreshMe()
  checked.value = true
  if (auth.isCoach) load()
})

async function load() {
  try {
    const rows = await request({ url: '/api/v1/coach/schedule' })
    const byDay = {}
    ;(rows || []).forEach((r) => {
      ;(byDay[r.weekday] = byDay[r.weekday] || []).push({
        startMinute: r.startMinute,
        endMinute: r.endMinute,
        s: fmtM(r.startMinute),
        e: fmtM(r.endMinute)
      })
    })
    items.value = DAY_ORDER.map((wd) => ({ weekday: wd, label: WEEK[wd], windows: byDay[wd] || [] }))
  } catch (e) {
    // toast 已由请求层处理
  }
  loadLeaves()
}

// ---- 时段编辑 ----
function addWindow(wd) {
  const day = items.value.find((x) => x.weekday === wd)
  if (day) day.windows.push({ startMinute: 540, endMinute: 1260, s: '09:00', e: '21:00' })
}

function removeWindow(wd, idx) {
  const day = items.value.find((x) => x.weekday === wd)
  if (day) day.windows.splice(idx, 1)
}

function onTime(wd, idx, field, value) {
  const day = items.value.find((x) => x.weekday === wd)
  const w = day && day.windows[idx]
  if (!w) return
  if (field === 's') {
    w.s = value
    w.startMinute = toMin(value)
  } else {
    w.e = value
    w.endMinute = toMin(value)
  }
}

// 保存全量周模板
async function onSave() {
  if (saving.value) return
  const list = []
  for (const d of items.value) {
    for (const w of d.windows) {
      if (w.startMinute >= w.endMinute) {
        uni.showToast({ title: `${d.label} ${w.s}-${w.e} 结束需晚于开始`, icon: 'none' })
        return
      }
      if (w.startMinute % 30 !== 0 || w.endMinute % 30 !== 0) {
        uni.showToast({ title: `${d.label} ${w.s}-${w.e} 需为 30 分钟粒度`, icon: 'none' })
        return
      }
      list.push({ weekday: d.weekday, startMinute: w.startMinute, endMinute: w.endMinute })
    }
  }
  saving.value = true
  try {
    await request({ url: '/api/v1/coach/schedule', method: 'PUT', data: { items: list } })
    uni.showToast({ title: '已保存', icon: 'success' })
  } catch (e) {
    // toast 已由请求层处理
  } finally {
    saving.value = false
  }
}

// ---- 请假管理 ----
async function loadLeaves() {
  try {
    const from = todayStr()
    const rows = await request({ url: `/api/v1/coach/leaves?from=${from}&to=${addDays(from, 60)}` })
    ;(rows || []).forEach((l) => {
      l.rangeText = `${fmtM(l.startMinute)}-${fmtM(l.endMinute)}`
      l.fullDay = l.startMinute === 0 && l.endMinute === 1440
    })
    leaves.value = rows || []
  } catch (e) {
    // toast 已由请求层处理
  }
}

async function onAddLeave() {
  if (addingLeave.value) return
  if (!leaveDate.value) {
    uni.showToast({ title: '请选择请假日期', icon: 'none' })
    return
  }
  const sm = toMin(leaveStart.value)
  const em = toMin(leaveEnd.value)
  if (sm >= em) {
    uni.showToast({ title: '结束时间需晚于开始时间', icon: 'none' })
    return
  }
  addingLeave.value = true
  try {
    const res = await request({
      url: '/api/v1/coach/leaves',
      method: 'POST',
      data: { date: leaveDate.value, startMinute: sm, endMinute: em, reason: '' }
    })
    if (res && res.affectedBookings > 0) {
      leaveAffected.value = res.affectedBookings
    } else {
      uni.showToast({ title: '已登记', icon: 'success' })
    }
    loadLeaves()
  } catch (e) {
    // toast 已由请求层处理
  } finally {
    addingLeave.value = false
  }
}

function askRemoveLeave(l) {
  removeLeave.value = l
}

async function doRemoveLeave() {
  const l = removeLeave.value
  if (!l || removing.value) return
  removing.value = true
  try {
    await request({ url: `/api/v1/coach/leaves/${l.id}`, method: 'DELETE' })
    removeLeave.value = null
    uni.showToast({ title: '已撤销', icon: 'success' })
    loadLeaves()
  } catch (e) {
    // toast 已由请求层处理
  } finally {
    removing.value = false
  }
}

function goApply() {
  uni.navigateTo({ url: '/pages/coach/apply' })
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
  margin-bottom: 12px;
  border-radius: $md-shape-md;
  background: var(--md-surface-container-high);
}

// 说明
.tip {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 12px;
  margin-bottom: 12px;
  border-radius: $md-shape-md;
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);
  &__icon {
    flex-shrink: 0;
    margin-top: 1px;
  }
  &__text {
    @include md-body-small;
    line-height: 18px;
  }
}

// 日卡
.day-card {
  margin-bottom: 12px;
  padding: 14px 16px;
}
.day-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.day-name {
  @include md-title-medium;
  font-weight: 600;
  color: var(--md-on-surface);
}
.win {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 10px;
}
.time-chip {
  padding: 8px 16px;
  border-radius: $md-shape-md;
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface);
  @include md-body-medium;
  cursor: pointer;
  &:active {
    opacity: 0.8;
  }
}
.sep {
  @include md-body-small;
  color: var(--md-on-surface-variant);
}
.win-del {
  margin-left: auto;
  color: var(--md-error);
}
.win-empty {
  display: block;
  margin-top: 8px;
  padding: 6px 0;
  @include md-body-small;
  color: var(--md-outline);
}

// 请假卡
.leave-card {
  margin-top: 16px;
  padding: 14px 16px;
}
.section-title {
  @include md-title-medium;
  font-weight: 600;
  color: var(--md-on-surface);
  margin-bottom: 6px;
}
.tip2 {
  display: block;
  @include md-body-small;
  color: var(--md-on-surface-variant);
  line-height: 18px;
  margin-bottom: 12px;
}
.leave-form {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.leave-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0 2px;
  border-bottom: 1px solid var(--md-outline-variant);
  &:last-of-type {
    border-bottom: none;
  }
}
.leave-text {
  @include md-body-medium;
  color: var(--md-on-surface);
}

// 底部保存栏
.footer-bar {
  position: fixed;
  bottom: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 100%;
  max-width: #{$md-page-max-width};
  padding: 12px 16px calc(12px + env(safe-area-inset-bottom));
  background: var(--md-surface-container);
  box-sizing: border-box;
  z-index: 10;
}
.footer-space {
  height: 76px;
}

.dialog-text {
  display: block;
  line-height: 20px;
}
</style>
