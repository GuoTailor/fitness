<template>
  <view class="md-page">
    <M3TopAppBar title="我的课包" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 待评价提示条 -->
      <view v-if="pending.length" class="pend-banner" @click="goReview">
        <M3Icon name="reviews" :size="20" class="pend-icon" />
        <view class="pend-info">
          <text class="pend-title">有 {{ pending.length }} 节课待评价</text>
          <text class="pend-sub md-ellipsis">{{ pending[0].projectName }} 等</text>
        </view>
        <text class="pend-go">去评价</text>
      </view>

      <M3Tabs :tabs="TABS" v-model="tab" class="page-tabs" />

      <!-- ==================== 课包 ==================== -->
      <template v-if="tab === 'packages'">
        <!-- 骨架 -->
        <template v-if="pkgLoading && !pkgs.length">
          <M3Card v-for="i in 2" :key="i" variant="filled" class="pkg-card sk-card">
            <M3Skeleton variant="text" width="150" :height="16" />
            <view class="sk-nums">
              <M3Skeleton v-for="j in 4" :key="j" variant="rect" :width="60" :height="36" />
            </view>
            <M3Skeleton variant="rect" :height="4" />
          </M3Card>
        </template>

        <!-- 空态 -->
        <M3EmptyState
          v-else-if="!pkgs.length"
          icon="shopping_bag"
          title="还没有课包"
          description="购买课包后即可开始上课"
        >
          <M3Button variant="tonal" icon="shopping_cart" @click="goBuy">去逛逛</M3Button>
        </M3EmptyState>

        <!-- 课包列表 -->
        <template v-else>
          <M3Card
            v-for="p in pkgs"
            :key="p.id"
            variant="filled"
            clickable
            class="pkg-card"
            @click="openTxns(p)"
          >
            <view class="pkg-head">
              <text class="pkg-title md-ellipsis">{{ p.packageTitle }}</text>
              <view class="pkg-status" :class="'tone-' + (STATUS_TONE[p.status] || 'muted')">
                <view class="pkg-status-dot" />
                <text>{{ p.statusText }}</text>
              </view>
            </view>

            <view v-if="p.status === 'inactive'" class="pkg-hint">首次上课自动激活</view>

            <view class="pkg-nums">
              <view class="pkg-num">
                <text class="pkg-num-v is-primary">{{ p.remaining }}</text>
                <text class="pkg-num-l">剩余课时</text>
              </view>
              <view class="pkg-num">
                <text class="pkg-num-v">{{ p.usedSessions }}</text>
                <text class="pkg-num-l">已消耗</text>
              </view>
              <view class="pkg-num">
                <text class="pkg-num-v">{{ p.totalSessions }}</text>
                <text class="pkg-num-l">总课时</text>
              </view>
              <view class="pkg-num">
                <text class="pkg-num-v pkg-num-date">{{ p.expireText }}</text>
                <text class="pkg-num-l">有效期至</text>
              </view>
            </view>

            <M3Progress type="linear" :model-value="p.percent" class="pkg-bar" />

            <view class="pkg-foot">
              <text class="pkg-foot-label">消费流水</text>
              <M3Icon name="expand_more" :size="20" class="pkg-foot-icon" />
            </view>
          </M3Card>
        </template>
      </template>

      <!-- ==================== 打卡 ==================== -->
      <template v-else-if="tab === 'checkin'">
        <!-- 打卡挑战 -->
        <M3Card v-if="checkin.enabled" variant="filled" class="ck-card">
          <view class="ck-head">
            <view class="ck-title-row">
              <M3Icon name="local_fire_department" :size="20" class="ck-icon" />
              <text class="sec-title">本月打卡挑战</text>
            </view>
            <view class="ck-badge" :class="ckBadgeClass">{{ ckBadgeText }}</view>
          </view>
          <view class="ck-progress">
            <view class="ck-bar-wrap">
              <M3Progress type="linear" :model-value="checkin.percent || 0" />
            </view>
            <text class="ck-num">{{ checkin.count }}/{{ checkin.target }} 次</text>
          </view>
          <text class="ck-sub">{{ checkinSub }}</text>
          <M3Button
            v-if="checkin.claimable"
            variant="filled"
            icon="redeem"
            :loading="claiming"
            class="ck-btn"
            @click="onClaim"
          >领取奖励券</M3Button>
        </M3Card>

        <!-- 训练日历 -->
        <M3Card variant="filled" class="cal-card">
          <view class="cal-head">
            <text class="sec-title">训练日历</text>
            <view class="cal-nav">
              <M3IconButton icon="arrow_back" :size="20" @click="shiftMonth(-1)" />
              <text class="cal-month">{{ month }}</text>
              <M3IconButton icon="arrow_forward" :size="20" @click="shiftMonth(1)" />
            </view>
          </view>
          <view class="cal-week-row">
            <text v-for="w in WEEKS" :key="w" class="cal-wd">{{ w }}</text>
          </view>
          <view class="cal-grid">
            <view v-for="c in cells" :key="c.key" class="cal-cell">
              <template v-if="!c.blank">
                <view class="cal-day" :class="{ 'is-today': c.today }">{{ c.day }}</view>
                <view v-if="c.count" class="cal-dot" />
              </template>
            </view>
          </view>
          <view class="cal-legend">
            <view class="cal-dot" />
            <text>当天有训练记录</text>
          </view>
        </M3Card>
      </template>

      <!-- ==================== 记录 ==================== -->
      <template v-else>
        <!-- 统计卡 -->
        <view v-if="stats" class="stat-grid">
          <view class="stat-cell">
            <text class="stat-v">{{ stats.week.lessons }}</text>
            <text class="stat-l">本周上课</text>
          </view>
          <view class="stat-cell">
            <text class="stat-v">{{ stats.week.minutes }}</text>
            <text class="stat-l">本周分钟</text>
          </view>
          <view class="stat-cell">
            <text class="stat-v">{{ stats.month.lessons }}</text>
            <text class="stat-l">本月上课</text>
          </view>
          <view class="stat-cell">
            <text class="stat-v">{{ stats.month.minutes }}</text>
            <text class="stat-l">本月分钟</text>
          </view>
        </view>

        <!-- 训练周报入口 -->
        <M3Card variant="filled" clickable class="report-entry" @click="goReport">
          <M3Icon name="insights" :size="22" class="report-entry-icon" />
          <view class="report-entry-info">
            <text class="report-entry-title">训练周报</text>
            <text class="report-entry-sub">每周训练量、教练分布与体重变化</text>
          </view>
          <M3Icon name="chevron_right" :size="24" class="report-entry-go" />
        </M3Card>

        <!-- 体重记录 -->
        <M3Card variant="filled" class="block-card">
          <view class="card-title-row">
            <M3Icon name="monitor_weight" :size="20" class="card-title-icon" />
            <text class="sec-title">体重记录</text>
            <text v-if="latestWeight" class="card-title-extra">当前 {{ latestWeight }}kg</text>
          </view>
          <view class="w-input-row">
            <M3TextField
              v-model="weightKg"
              label="今日体重"
              placeholder="记录今日体重（kg）"
              type="number"
              class="w-input"
              label-bg="var(--md-surface-container-high)"
              @confirm="saveWeight"
            />
            <M3Button variant="filled" :loading="savingWeight" @click="saveWeight">记录</M3Button>
          </view>
          <view v-if="weightBars.length > 1" class="w-chart">
            <view v-for="(b, i) in weightBars" :key="i" class="w-col">
              <view class="w-bar" :style="{ height: b.pct + '%' }" />
            </view>
          </view>
          <text v-else class="block-empty">记录两次以上体重后展示趋势曲线</text>
        </M3Card>

        <!-- 教练课后反馈 -->
        <M3Card v-if="feedbacks.length" variant="filled" class="block-card">
          <view class="card-title-row">
            <M3Icon name="chat_bubble" :size="20" class="card-title-icon" />
            <text class="sec-title">教练课后反馈</text>
          </view>
          <template v-for="(f, i) in feedbacks" :key="i">
            <M3Divider v-if="i > 0" class="fb-divider" />
            <view class="fb-item">
              <view class="fb-head">
                <text class="fb-coach">{{ f.coachName }} 教练</text>
                <text class="fb-meta">{{ f.typeText }} · {{ f.dateText }}</text>
              </view>
              <text class="fb-content">{{ f.feedback && f.feedback.content }}</text>
            </view>
          </template>
        </M3Card>

        <!-- 训练记录 -->
        <M3Card variant="filled" class="block-card">
          <view class="card-title-row">
            <M3Icon name="history" :size="20" class="card-title-icon" />
            <text class="sec-title">训练记录</text>
          </view>
          <template v-if="records.length">
            <template v-for="(r, i) in records" :key="r.id">
              <M3Divider v-if="i > 0" inset class="rec-divider" />
              <view class="rec-row">
                <view class="rec-info">
                  <text class="rec-name md-ellipsis">{{ r.project }}</text>
                  <text class="rec-sub">{{ r.recordDate }} · {{ r.sourceText }}</text>
                </view>
                <view class="rec-dur">
                  <M3Icon name="timer" :size="14" class="rec-dur-icon" />
                  <text>{{ r.durationMinutes }} 分钟</text>
                </view>
              </view>
            </template>
          </template>
          <text v-else class="block-empty">完成上课后自动生成记录</text>
        </M3Card>
      </template>
    </view>

    <!-- 消费流水弹层 -->
    <M3BottomSheet :open="sheetOpen" :title="txnTitle" @close="sheetOpen = false">
      <view v-if="txnLoading" class="txn-state">
        <M3Progress type="linear" indeterminate />
      </view>
      <template v-else-if="txns.length">
        <view v-for="t in txns" :key="t.id" class="txn-row">
          <view class="txn-info">
            <text class="txn-type">{{ t.typeText }}</text>
            <text class="txn-meta">{{ t.timeText }}{{ t.remark ? ' · ' + t.remark : '' }}</text>
          </view>
          <text class="txn-val" :class="t.sessionsChange > 0 ? 'is-pos' : 'is-neg'">
            {{ t.sessionsChange > 0 ? '+' : '' }}{{ t.sessionsChange }}
          </text>
        </view>
      </template>
      <text v-else class="txn-state txn-empty-text">暂无流水</text>
    </M3BottomSheet>
  </view>
</template>

<script setup>
import { ref, computed } from 'vue'
import { onLoad, onShow, onPageScroll } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
import { fmtTime } from '@/utils/format'

const auth = useAuthStore()

const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

const TABS = [
  { value: 'packages', label: '课包' },
  { value: 'checkin', label: '打卡' },
  { value: 'records', label: '记录' }
]
const tab = ref('packages')
const WEEKS = ['日', '一', '二', '三', '四', '五', '六']

// 课包状态 → 语义色（ok=primary / info=tertiary / muted=outline / bad=error）
const STATUS_TONE = {
  inactive: 'info',
  active: 'ok',
  exhausted: 'muted',
  expired: 'bad',
  refunded: 'bad'
}
const TXN_TYPE_TEXT = {
  purchase: '购买入账',
  deduct: '上课扣除',
  booking_cancel: '取消返还',
  penalty: '取消扣次',
  refund_reclaim: '退款回收',
  manual_adjust: '手动调整',
  gift: '赠课入账'
}

function isoDate(d) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

onLoad(async () => {
  await auth.ensureLogin().catch(() => {})
})

onShow(() => {
  if (!auth.token) return
  loadPkgs()
  loadCheckin()
  loadTraining()
  loadWeight()
  loadFeedbacks()
  loadPending()
  if (!month.value) {
    month.value = isoDate(new Date()).slice(0, 7)
    loadCalendar(month.value)
  }
})

// ---------- 待评价 ----------
const pending = ref([])

async function loadPending() {
  try {
    const d = await request({ url: '/api/v1/reviews/reviewable' })
    pending.value = (d.items || []).map((x) => ({
      ...x,
      dateText: String(x.verifiedAt || '').slice(0, 10)
    }))
  } catch (e) { /* 弱网容忍 */ }
}

function goReview() {
  uni.navigateTo({ url: '/pages/review/create' })
}

// ---------- 课包 ----------
const pkgs = ref([])
const pkgLoading = ref(false)

async function loadPkgs() {
  pkgLoading.value = true
  try {
    const list = await request({ url: '/api/v1/member/packages' })
    pkgs.value = (list || []).map((p) => ({
      ...p,
      statusText: {
        inactive: '未开卡', active: '生效中', exhausted: '已用完',
        expired: '已过期', refunded: '已退款'
      }[p.status] || p.status,
      remaining: p.totalSessions - p.usedSessions,
      percent: p.totalSessions ? Math.round((p.usedSessions / p.totalSessions) * 100) : 0,
      expireText: p.expireDate ? String(p.expireDate).slice(0, 10) : '—'
    }))
  } catch (e) {
    // 错误已由请求层 toast
  } finally {
    pkgLoading.value = false
  }
}

// 消费流水（底部弹层）
const sheetOpen = ref(false)
const txnTitle = ref('')
const txns = ref([])
const txnLoading = ref(false)

async function openTxns(p) {
  txnTitle.value = p.packageTitle || '消费流水'
  sheetOpen.value = true
  txnLoading.value = true
  txns.value = []
  try {
    const list = await request({ url: `/api/v1/member/packages/${p.id}/transactions` })
    txns.value = (list || []).map((t) => ({
      ...t,
      typeText: TXN_TYPE_TEXT[t.type] || t.type,
      timeText: fmtTime(t.createdAt)
    }))
  } catch (e) {
    txns.value = []
  } finally {
    txnLoading.value = false
  }
}

function goBuy() {
  uni.reLaunch({ url: '/pages/course/list' })
}

// ---------- 打卡挑战 ----------
const checkin = ref({ enabled: false })
const claiming = ref(false)

const ckBadgeText = computed(() =>
  checkin.value.claimed ? '已领取' : checkin.value.claimable ? '可领取' : '进行中'
)
const ckBadgeClass = computed(() =>
  checkin.value.claimed ? 'is-done' : checkin.value.claimable ? 'is-ready' : ''
)
const checkinSub = computed(() => {
  const c = checkin.value
  if (c.claimed) return '本月奖励已到账，次月再来'
  if (c.claimable) return '达标啦，领取本月奖励券'
  if (c.remaining) return `再上 ${c.remaining} 次课即可领奖励券`
  return '坚持上课，达标即送奖励券'
})

async function loadCheckin() {
  try {
    const data = await request({ url: '/api/v1/checkin/challenge' })
    if (data.enabled && data.target) {
      data.percent = Math.min(100, Math.round((data.count / data.target) * 100))
    }
    checkin.value = data || { enabled: false }
  } catch (e) { /* 静默 */ }
}

async function onClaim() {
  claiming.value = true
  try {
    await request({ url: '/api/v1/checkin/claim', method: 'POST', data: {} })
    uni.showToast({ title: '奖励已领取', icon: 'success' })
    loadCheckin()
  } catch (e) {
    // toast 已处理
  } finally {
    claiming.value = false
  }
}

// ---------- 训练统计 / 记录 ----------
const stats = ref(null)
const records = ref([])

async function loadTraining() {
  try {
    const tr = await request({ url: '/api/v1/training' })
    stats.value = tr.stats || null
    records.value = (tr.records || []).map((r) => ({
      ...r,
      sourceText: r.source === 'group' ? '团课' : '私教'
    }))
  } catch (e) { /* 错误已由请求层 toast */ }
}

function goReport() {
  uni.navigateTo({ url: '/pages/training/report' })
}

// ---------- 训练日历 ----------
const month = ref('')
const cells = ref([])

async function loadCalendar(m) {
  try {
    const cal = await request({ url: '/api/v1/training/calendar', data: { month: m } })
    const map = {}
    ;((cal && cal.days) || []).forEach((x) => { map[x.date] = x.count })
    const [y, mo] = m.split('-').map(Number)
    const firstDow = new Date(y, mo - 1, 1).getDay()
    const daysInMonth = new Date(y, mo, 0).getDate()
    const today = isoDate(new Date())
    const arr = []
    for (let i = 0; i < firstDow; i++) arr.push({ blank: true, key: 'b' + i })
    for (let d = 1; d <= daysInMonth; d++) {
      const ds = `${m}-${String(d).padStart(2, '0')}`
      arr.push({ key: ds, day: d, count: map[ds] || 0, today: ds === today })
    }
    cells.value = arr
  } catch (e) { /* 静默 */ }
}

function shiftMonth(delta) {
  const [y, mo] = month.value.split('-').map(Number)
  const d = new Date(y, mo - 1 + delta, 1)
  month.value = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
  loadCalendar(month.value)
}

// ---------- 体重记录 ----------
const weightKg = ref('')
const latestWeight = ref('')
const weightBars = ref([])
const savingWeight = ref(false)

async function loadWeight() {
  try {
    const rows = await request({ url: '/api/v1/body-metrics', data: { limit: 30 } })
    const bars = []
    if (rows && rows.length) {
      const vals = rows.map((r) => r.weightG)
      const min = Math.min(...vals)
      const max = Math.max(...vals)
      const span = Math.max(max - min, 1)
      rows.forEach((r) => {
        bars.push({
          pct: Math.round(15 + ((r.weightG - min) / span) * 85),
          date: String(r.recordDate).slice(5)
        })
      })
      latestWeight.value = (rows[rows.length - 1].weightG / 1000).toFixed(1)
    } else {
      latestWeight.value = ''
    }
    weightBars.value = bars
  } catch (e) { /* 静默 */ }
}

async function saveWeight() {
  const kg = parseFloat(weightKg.value)
  if (!kg || kg < 20 || kg > 300) {
    uni.showToast({ title: '请输入 20~300 之间的体重', icon: 'none' })
    return
  }
  savingWeight.value = true
  try {
    await request({
      url: '/api/v1/body-metrics',
      method: 'POST',
      data: { date: isoDate(new Date()), weightG: Math.round(kg * 1000) }
    })
    uni.showToast({ title: '已记录', icon: 'success' })
    weightKg.value = ''
    loadWeight()
  } catch (e) {
    // toast 已处理
  } finally {
    savingWeight.value = false
  }
}

// ---------- 教练课后反馈 ----------
const feedbacks = ref([])

async function loadFeedbacks() {
  try {
    const rows = await request({ url: '/api/v1/feedback/mine' })
    feedbacks.value = (rows || []).map((x) => ({
      ...x,
      dateText: fmtTime((x.feedback && x.feedback.createdAt) || ''),
      typeText: x.bizType === 'group' ? '团课' : '私教'
    }))
  } catch (e) { /* 静默 */ }
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

.page-tabs {
  margin-bottom: 14px;
}

.sec-title {
  @include md-title-medium;
  color: var(--md-on-surface);
}

// ============ 待评价提示条 ============
.pend-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  margin-bottom: 12px;
  border-radius: $md-shape-md;
  background: var(--md-primary-container);
  color: var(--md-on-primary-container);
  cursor: pointer;

  &:active {
    opacity: 0.88;
  }
}

.pend-icon {
  flex-shrink: 0;
}

.pend-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.pend-title {
  @include md-title-small;
  font-weight: 600;
}

.pend-sub {
  @include md-body-small;
  opacity: 0.8;
}

.pend-go {
  @include md-label-large;
  font-weight: 600;
  flex-shrink: 0;
}

// ============ 课包 ============
.pkg-card {
  padding: 16px;
  margin-bottom: 12px;
}

.sk-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.sk-nums {
  display: flex;
  justify-content: space-between;
}

.pkg-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.pkg-title {
  flex: 1;
  min-width: 0;
  @include md-title-medium;
  color: var(--md-on-surface);
}

.pkg-status {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  @include md-label-small;

  &-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
  }

  &.tone-ok {
    color: var(--md-primary);
  }
  &.tone-info {
    color: var(--md-tertiary);
  }
  &.tone-muted {
    color: var(--md-outline);
  }
  &.tone-bad {
    color: var(--md-error);
  }
}

.pkg-hint {
  margin-top: 6px;
  @include md-body-small;
  color: var(--md-tertiary);
}

.pkg-nums {
  display: flex;
  margin: 14px 0 12px;
}

.pkg-num {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.pkg-num-v {
  @include md-title-medium;
  font-weight: 700;
  color: var(--md-on-surface);

  &.is-primary {
    @include md-title-large;
    color: var(--md-primary);
  }

  &.pkg-num-date {
    @include md-label-medium;
    font-weight: 500;
    padding-top: 6px;
  }
}

.pkg-num-l {
  @include md-label-small;
  color: var(--md-on-surface-variant);
}

.pkg-bar {
  margin-bottom: 10px;
}

.pkg-foot {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding-top: 6px;
  border-top: 1px solid var(--md-outline-variant);
}

.pkg-foot-label {
  @include md-label-medium;
  color: var(--md-primary);
}

.pkg-foot-icon {
  color: var(--md-primary);
}

// ============ 打卡挑战 ============
.ck-card {
  padding: 16px;
  margin-bottom: 12px;
}

.ck-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.ck-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ck-icon {
  color: var(--md-error);
}

.ck-badge {
  @include md-label-medium;
  padding: 2px 10px;
  border-radius: $md-shape-full;
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);

  &.is-ready {
    background: var(--md-primary);
    color: var(--md-on-primary);
  }
  &.is-done {
    background: var(--md-primary-container);
    color: var(--md-on-primary-container);
  }
}

.ck-progress {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 14px;
}

.ck-bar-wrap {
  flex: 1;
  min-width: 0;
}

.ck-num {
  @include md-label-large;
  color: var(--md-on-surface);
  flex-shrink: 0;
}

.ck-sub {
  display: block;
  margin-top: 10px;
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

.ck-btn {
  margin-top: 14px;
}

// ============ 训练日历 ============
.cal-card {
  padding: 16px;
  margin-bottom: 12px;
}

.cal-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.cal-nav {
  display: flex;
  align-items: center;
  gap: 4px;
}

.cal-month {
  @include md-title-small;
  color: var(--md-on-surface);
  min-width: 88px;
  text-align: center;
}

.cal-week-row {
  display: flex;
  margin-top: 12px;
}

.cal-wd {
  width: 14.28%;
  text-align: center;
  @include md-label-small;
  color: var(--md-on-surface-variant);
  padding: 4px 0;
}

.cal-grid {
  display: flex;
  flex-wrap: wrap;
}

.cal-cell {
  width: 14.28%;
  height: 44px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.cal-day {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  @include md-label-medium;
  color: var(--md-on-surface);

  &.is-today {
    background: var(--md-primary-container);
    color: var(--md-on-primary-container);
    font-weight: 600;
  }
}

.cal-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--md-primary);
  margin-top: 1px;
}

.cal-legend {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  margin-top: 8px;
  @include md-label-small;
  color: var(--md-on-surface-variant);
}

// ============ 记录 ============
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
  gap: 2px;
}

.stat-v {
  @include md-headline-small;
  font-weight: 600;
  color: var(--md-primary);
}

.stat-l {
  @include md-label-medium;
  color: var(--md-on-surface-variant);
}

.report-entry {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
  margin-bottom: 12px;
}

.report-entry-icon {
  color: var(--md-primary);
  flex-shrink: 0;
}

.report-entry-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.report-entry-title {
  @include md-title-medium;
  color: var(--md-on-surface);
}

.report-entry-sub {
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

.report-entry-go {
  color: var(--md-on-surface-variant);
  flex-shrink: 0;
}

.block-card {
  padding: 16px;
  margin-bottom: 12px;
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

.card-title-extra {
  flex: 1;
  text-align: right;
  @include md-label-medium;
  color: var(--md-primary);
  font-weight: 600;
}

.block-empty {
  display: block;
  padding: 8px 0 2px;
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

// ---- 体重 ----
.w-input-row {
  display: flex;
  align-items: center;
  gap: 10px;

  .w-input {
    flex: 1;
  }

  > :last-child {
    flex-shrink: 0;
  }
}

.w-chart {
  display: flex;
  align-items: flex-end;
  gap: 4px;
  height: 96px;
  margin-top: 16px;
  padding: 8px;
  border-radius: $md-shape-sm;
  background: var(--md-surface-container-lowest);
}

.w-col {
  flex: 1;
  height: 100%;
  display: flex;
  align-items: flex-end;
}

.w-bar {
  width: 100%;
  border-radius: $md-shape-xs $md-shape-xs 0 0;
  background: var(--md-primary);
  opacity: 0.75;
  transition: height $md-motion-medium $md-motion-standard;
}

// ---- 反馈 ----
.fb-divider {
  margin: 12px 0;
}

.fb-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}

.fb-coach {
  @include md-title-small;
  color: var(--md-on-surface);
  flex-shrink: 0;
}

.fb-meta {
  @include md-label-small;
  color: var(--md-on-surface-variant);
}

.fb-content {
  display: block;
  margin-top: 6px;
  @include md-body-medium;
  color: var(--md-on-surface-variant);
  line-height: 21px;
}

// ---- 训练记录 ----
.rec-divider {
  margin: 4px 0;
}

.rec-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 0;
}

.rec-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.rec-name {
  @include md-body-large;
  color: var(--md-on-surface);
}

.rec-sub {
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

.rec-dur {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  @include md-label-medium;
  color: var(--md-primary);
}

.rec-dur-icon {
  color: var(--md-primary);
}

// ============ 消费流水弹层 ============
.txn-state {
  padding: 16px 0;
}

.txn-empty-text {
  display: block;
  text-align: center;
  @include md-body-medium;
  color: var(--md-on-surface-variant);
}

.txn-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0;

  & + .txn-row {
    border-top: 1px solid var(--md-outline-variant);
  }
}

.txn-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.txn-type {
  @include md-body-large;
  color: var(--md-on-surface);
}

.txn-meta {
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

.txn-val {
  @include md-title-medium;
  font-weight: 700;
  flex-shrink: 0;

  &.is-pos {
    color: var(--md-primary);
  }
  &.is-neg {
    color: var(--md-on-surface-variant);
  }
}
</style>
