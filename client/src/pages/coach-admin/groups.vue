<template>
  <view class="md-page">
    <M3TopAppBar title="团课管理" :back="true" :elevated="scrolled" />

    <!-- 身份门槛 -->
    <view v-if="!checked" class="md-shell page-body">
      <M3Card v-for="i in 3" :key="i" variant="filled" class="sk-card">
        <view class="sk-inner">
          <M3Skeleton variant="text" width="55%" height="16" />
          <M3Skeleton variant="text" width="80%" height="14" />
        </view>
      </M3Card>
    </view>

    <M3EmptyState
      v-else-if="!auth.isCoach"
      icon="manage_accounts"
      title="需要教练身份"
      description="登录教练账号后即可管理团课"
    >
      <M3Button variant="filled" icon="how_to_reg" @click="goApply">去入驻</M3Button>
    </M3EmptyState>

    <view v-else class="md-shell page-body page-body--fab">
      <!-- 骨架 -->
      <template v-if="loading && !list.length">
        <M3Card v-for="i in 3" :key="i" variant="filled" class="sk-card">
          <view class="sk-inner">
            <M3Skeleton variant="text" width="55%" height="16" />
            <M3Skeleton variant="text" width="80%" height="14" />
          </view>
        </M3Card>
      </template>

      <!-- 团课列表 -->
      <template v-else-if="list.length">
        <M3Card v-for="g in list" :key="g.id" variant="filled" class="gc-card">
          <view class="gc-inner">
            <view class="gc-head">
              <text class="gc-title md-ellipsis">{{ g.title }}</text>
              <view class="st" :style="{ color: statusOf(g.status)[1] }">
                <view class="st__dot" :style="{ background: statusOf(g.status)[1] }" />
                <text class="st__label">{{ statusOf(g.status)[0] }}</text>
              </view>
            </view>
            <text class="gc-sub md-ellipsis">{{ g.timeText }} · {{ g.location || '待定' }} · {{ g.enrolledCount }}/{{ g.capacity }} 人</text>
            <view class="gc-ops">
              <M3Button variant="tonal" size="sm" icon="groups" @click="openEnrollments(g)">名单</M3Button>
              <M3Button
                v-if="g.status === 'enrolling' || g.status === 'fulfilled'"
                variant="outlined"
                size="sm"
                danger
                @click="askCancel(g)"
              >取消</M3Button>
            </view>
          </view>
        </M3Card>
      </template>

      <!-- 空态 -->
      <M3EmptyState
        v-else
        icon="groups"
        title="暂未发布团课"
        description="发布团课后学员可在课表中报名"
      >
        <M3Button variant="filled" icon="add" @click="openForm">发布团课</M3Button>
      </M3EmptyState>
    </view>

    <!-- 发布团课 FAB -->
    <view v-if="checked && auth.isCoach" class="fab-wrap">
      <M3Fab icon="add" label="发布团课" @click="openForm" />
    </view>

    <!-- 发布团课弹层 -->
    <M3BottomSheet :open="formOpen" title="发布团课" @close="formOpen = false">
      <view class="form">
        <!-- 选课包 -->
        <picker mode="selector" :range="pkgLabels" @change="(e) => onPkg(e.detail.value)">
          <view class="form-row">
            <text class="form-label">课包</text>
            <text class="form-value md-ellipsis" :class="{ 'is-muted': !selectedPkg }">{{ selectedPkg ? selectedPkg.title : '请选择' }}</text>
            <M3Icon name="expand_more" :size="20" class="form-go" />
          </view>
        </picker>

        <M3TextField
          v-model="form.title"
          label="团课标题"
          placeholder="如：燃脂训练营"
          :maxlength="30"
          class="form-field"
        />

        <view class="form-grid">
          <picker mode="date" :value="form.date" :start="tomorrowStr()" @change="(e) => (form.date = e.detail.value)">
            <view class="time-chip">{{ form.date }}</view>
          </picker>
          <picker mode="time" :value="form.s" @change="(e) => onFormTime('s', e.detail.value)">
            <view class="time-chip">{{ form.s }}</view>
          </picker>
          <text class="sep">至</text>
          <picker mode="time" :value="form.e" @change="(e) => onFormTime('e', e.detail.value)">
            <view class="time-chip">{{ form.e }}</view>
          </picker>
        </view>

        <M3TextField
          v-model="form.location"
          label="地点"
          placeholder="如：1 号团课厅"
          :maxlength="30"
          class="form-field"
        />

        <M3TextField
          v-model="form.capacity"
          label="容量"
          placeholder="报名人数上限"
          type="number"
          :maxlength="4"
          class="form-field"
        />

        <M3Button variant="filled" block :loading="publishing" @click="onPublish">发布</M3Button>
      </view>
    </M3BottomSheet>

    <!-- 报名名单弹层 -->
    <M3BottomSheet :open="enrollOpen" title="报名名单" @close="enrollOpen = false">
      <view v-if="enrollLoading" class="enroll-loading">
        <M3Progress type="circular" indeterminate />
      </view>
      <template v-else-if="enrollList.length">
        <view v-for="(x, i) in enrollList" :key="i" class="enroll-row">
          <M3Avatar :name="x.userName || '学员'" :size="36" />
          <view class="enroll-info">
            <text class="enroll-name md-ellipsis">{{ x.userName || '学员' }}</text>
            <text class="enroll-phone">{{ x.userPhone || '无手机号' }}</text>
          </view>
          <M3Icon v-if="x.enrollment && x.enrollment.status === 'verified'" name="task_alt" :size="20" class="enroll-ok" />
        </view>
      </template>
      <M3EmptyState v-else icon="group_add" title="暂无报名" description="学员报名后会出现在这里" />
    </M3BottomSheet>

    <!-- 取消团课确认 -->
    <M3Dialog :open="!!cancelRow" title="取消团课" icon="warning" :dismissible="false">
      <text class="dialog-text">取消「{{ cancelRow ? cancelRow.title : '' }}」将退还全部已报名学员的课时，确定取消吗？</text>
      <template #actions>
        <M3Button variant="text" @click="cancelRow = null">再想想</M3Button>
        <M3Button variant="filled" danger :loading="cancelling" @click="doCancel">取消团课</M3Button>
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

const checked = ref(false)
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

const fmtM = (m) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`
const iso = (d) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
const tomorrowStr = () => iso(new Date(Date.now() + 86400000))

// 团课状态 → 文案 + MD3 语义色
const GC_STATUS = {
  enrolling: ['报名中', 'var(--md-primary)'],
  fulfilled: ['已满员', 'var(--md-tertiary)'],
  closed: ['已截止', 'var(--md-on-surface-variant)'],
  cancelled: ['已取消', 'var(--md-error)'],
  finished: ['已结束', 'var(--md-on-surface-variant)']
}
const statusOf = (s) => GC_STATUS[s] || [s, 'var(--md-on-surface-variant)']

const list = ref([])
const pkgs = ref([])
const loading = ref(false)

// 发布表单
const formOpen = ref(false)
const publishing = ref(false)
const form = ref(defaultForm())

function defaultForm() {
  return {
    packageId: 0,
    title: '',
    date: tomorrowStr(),
    s: '19:00',
    e: '20:00',
    startMinute: 1140,
    endMinute: 1200,
    location: '',
    capacity: '10'
  }
}

const selectedPkg = computed(() => pkgs.value.find((p) => p.id === form.value.packageId) || null)
const pkgLabels = computed(() => pkgs.value.map((p) => p.title))

// 报名名单
const enrollOpen = ref(false)
const enrollLoading = ref(false)
const enrollList = ref([])

// 取消团课
const cancelRow = ref(null)
const cancelling = ref(false)

onLoad(async () => {
  await auth.ensureLogin().catch(() => {})
  if (auth.token && !auth.isCoach) await auth.refreshMe()
  checked.value = true
  if (auth.isCoach) loadPkgs()
})

onShow(() => {
  if (checked.value && auth.isCoach) load()
})

async function load() {
  loading.value = true
  try {
    const rows = await request({ url: '/api/v1/coach/group-classes' })
    ;(rows || []).forEach((g) => {
      g.timeText = `${g.classDate} ${fmtM(g.startMinute)}-${fmtM(g.endMinute)}`
    })
    list.value = rows || []
  } catch (e) {
    // toast 已由请求层处理
  } finally {
    loading.value = false
  }
}

async function loadPkgs() {
  try {
    pkgs.value = (await request({ url: '/api/v1/coach/packages' })) || []
  } catch (e) {
    pkgs.value = []
  }
}

// ---- 发布 ----
function openForm() {
  form.value = defaultForm()
  if (pkgs.value.length) form.value.packageId = pkgs.value[0].id
  formOpen.value = true
}

function onPkg(idx) {
  const p = pkgs.value[Number(idx)]
  if (p) form.value.packageId = p.id
}

function onFormTime(field, value) {
  form.value[field] = value
  form.value[field === 's' ? 'startMinute' : 'endMinute'] =
    Number(value.split(':')[0]) * 60 + Number(value.split(':')[1])
}

async function onPublish() {
  const f = form.value
  if (!f.title.trim()) {
    uni.showToast({ title: '请填标题', icon: 'none' })
    return
  }
  if (!f.packageId) {
    uni.showToast({ title: '请选择课包', icon: 'none' })
    return
  }
  const cap = Number(f.capacity)
  if (!cap || cap < 1) {
    uni.showToast({ title: '请填写容量', icon: 'none' })
    return
  }
  if (f.startMinute >= f.endMinute) {
    uni.showToast({ title: '结束时间需晚于开始时间', icon: 'none' })
    return
  }
  publishing.value = true
  try {
    await request({
      url: '/api/v1/coach/group-classes',
      method: 'POST',
      data: {
        packageId: f.packageId,
        title: f.title.trim(),
        date: f.date,
        startMinute: f.startMinute,
        endMinute: f.endMinute,
        location: f.location.trim(),
        capacity: cap
      }
    })
    formOpen.value = false
    uni.showToast({ title: '已发布', icon: 'success' })
    load()
  } catch (e) {
    // toast 已由请求层处理
  } finally {
    publishing.value = false
  }
}

// ---- 报名名单 ----
async function openEnrollments(g) {
  enrollOpen.value = true
  enrollLoading.value = true
  enrollList.value = []
  try {
    enrollList.value = (await request({ url: `/api/v1/coach/group-classes/${g.id}/enrollments` })) || []
  } catch (e) {
    enrollOpen.value = false
  } finally {
    enrollLoading.value = false
  }
}

// ---- 取消团课 ----
function askCancel(g) {
  cancelRow.value = g
}

async function doCancel() {
  const g = cancelRow.value
  if (!g || cancelling.value) return
  cancelling.value = true
  try {
    await request({
      url: `/api/v1/coach/group-classes/${g.id}/cancel`,
      method: 'POST',
      data: { reason: '教练取消' }
    })
    cancelRow.value = null
    uni.showToast({ title: '已取消', icon: 'none' })
    load()
  } catch (e) {
    // toast 已由请求层处理
  } finally {
    cancelling.value = false
  }
}

function goApply() {
  uni.navigateTo({ url: '/pages/coach/apply' })
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
  &--fab {
    padding-bottom: 96px;
  }
}

// 骨架
.sk-card {
  margin-bottom: 12px;
}
.sk-inner {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

// 团课卡
.gc-card {
  margin-bottom: 12px;
}
.gc-inner {
  padding: 14px 16px;
}
.gc-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.gc-title {
  flex: 1;
  min-width: 0;
  @include md-title-medium;
  font-weight: 600;
  color: var(--md-on-surface);
}
.gc-sub {
  display: block;
  margin-top: 6px;
  @include md-body-small;
  color: var(--md-on-surface-variant);
}
.gc-ops {
  margin-top: 12px;
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

// FAB
.fab-wrap {
  position: fixed;
  right: 16px;
  bottom: calc(20px + env(safe-area-inset-bottom));
  z-index: 20;
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

// 发布表单
.form {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-bottom: 8px;
}
.form-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  border: 1px solid var(--md-outline);
  border-radius: $md-shape-md;
  cursor: pointer;
  &:active {
    background: rgba(0, 0, 0, $md-state-pressed-opacity);
  }
}
.form-label {
  @include md-label-medium;
  color: var(--md-on-surface-variant);
  flex-shrink: 0;
}
.form-value {
  flex: 1;
  min-width: 0;
  text-align: right;
  @include md-body-medium;
  color: var(--md-on-surface);
  &.is-muted {
    color: var(--md-outline);
  }
}
.form-go {
  color: var(--md-on-surface-variant);
  flex-shrink: 0;
}
.form-field {
  display: block;
}
.form-grid {
  display: flex;
  align-items: center;
  gap: 10px;
}
.time-chip {
  padding: 10px 14px;
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

// 报名名单
.enroll-loading {
  display: flex;
  justify-content: center;
  padding: 32px 0;
}
.enroll-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 4px;
}
.enroll-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.enroll-name {
  @include md-body-large;
  color: var(--md-on-surface);
}
.enroll-phone {
  @include md-body-small;
  color: var(--md-on-surface-variant);
}
.enroll-ok {
  color: var(--md-primary);
  flex-shrink: 0;
}

.dialog-text {
  display: block;
  line-height: 20px;
}
</style>
