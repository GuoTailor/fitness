<template>
  <view class="md-page">
    <M3TopAppBar title="核销" :back="true" :elevated="scrolled" />

    <!-- 身份门槛 -->
    <view v-if="!checked" class="md-shell page-body">
      <M3Skeleton variant="rect" height="56px" class="sk-gap" />
      <M3Skeleton variant="rect" height="120px" class="sk-gap" />
    </view>

    <M3EmptyState
      v-else-if="!auth.isCoach"
      icon="manage_accounts"
      title="需要教练身份"
      description="登录教练账号后即可扫码核销"
    >
      <M3Button variant="filled" icon="how_to_reg" @click="goApply">去入驻</M3Button>
    </M3EmptyState>

    <view v-else class="md-shell page-body">
      <!-- 扫码核销（仅小程序） -->
      <!-- #ifdef MP-WEIXIN -->
      <M3Button variant="filled" size="lg" block icon="qr_code_scanner" :loading="verifying" @click="onScan">
        扫一扫核销
      </M3Button>
      <!-- #endif -->

      <!-- 手动输码 -->
      <M3Card variant="filled" class="manual-card">
        <text class="manual-tip">学员未带手机？让学员报码，在此输入：</text>
        <view class="manual-row">
          <M3TextField
            v-model="manualCode"
            label="核销码"
            placeholder="输入核销码（V/G 开头）"
            :maxlength="32"
            class="manual-input"
            @confirm="onManualSubmit"
          />
          <M3Button variant="tonal" :loading="verifying" @click="onManualSubmit">核销</M3Button>
        </view>
      </M3Card>

      <!-- 成功反馈卡 -->
      <view v-if="lastResult" class="result-card is-ok">
        <view class="result-head">
          <M3Icon name="check_circle" :size="24" class="result-icon" />
          <text class="result-title">{{ lastResult.alreadyVerified ? '该码已核销过（幂等）' : '核销成功' }}</text>
        </view>
        <text class="result-line">
          {{ lastResult.bizType === 'private' ? '私教课' : '团课' }} · {{ lastResult.userName }} {{ lastResult.userPhone }}
        </text>
        <text v-if="lastResult.verifiedAt" class="result-sub">{{ fmtTime(lastResult.verifiedAt) }}</text>
      </view>

      <!-- 失败反馈卡 -->
      <view v-if="lastError" class="result-card is-bad">
        <view class="result-head">
          <M3Icon name="error" :size="24" class="result-icon" />
          <text class="result-title">{{ lastError }}</text>
        </view>
      </view>

      <!-- 核销记录（本次会话） -->
      <template v-if="records.length">
        <view class="rec-title">
          <M3Icon name="history" :size="20" class="rec-title__icon" />
          <text>核销记录</text>
        </view>
        <M3Card v-for="(r, i) in records" :key="i" variant="outlined" class="rec-card">
          <view class="rec-inner">
            <view class="rec-main">
              <text class="rec-name md-ellipsis">{{ r.userName }} · {{ r.bizType === 'private' ? '私教课' : '团课' }}</text>
              <text class="rec-meta">{{ r.code }} · {{ r.timeText }}</text>
            </view>
            <M3Icon :name="r.already ? 'info' : 'task_alt'" :size="20" :class="r.already ? 'rec-icon-warn' : 'rec-icon-ok'" />
          </view>
        </M3Card>
      </template>
    </view>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onPageScroll } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
import { fmtTime } from '@/utils/format'

const auth = useAuthStore()

const checked = ref(false)
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

const manualCode = ref('')
const verifying = ref(false)
const lastResult = ref(null)
const lastError = ref('')
const records = ref([])

onLoad(async () => {
  await auth.ensureLogin().catch(() => {})
  if (auth.token && !auth.isCoach) await auth.refreshMe()
  checked.value = true
})

// 扫码核销（仅微信小程序端）
// #ifdef MP-WEIXIN
async function onScan() {
  if (verifying.value) return
  try {
    const scan = await uni.scanCode({ onlyFromCamera: false })
    if (scan && scan.result) await doVerify(scan.result)
  } catch (e) {
    // 用户取消扫码
  }
}
// #endif

function onManualSubmit() {
  const code = manualCode.value.trim()
  if (!code || verifying.value) return
  doVerify(code)
}

async function doVerify(code) {
  verifying.value = true
  lastError.value = ''
  try {
    const r = await request({ url: '/api/v1/coach/verify', method: 'POST', data: { code } })
    lastResult.value = r
    if (!r.alreadyVerified) manualCode.value = ''
    // 震动反馈
    try {
      uni.vibrateShort({})
    } catch (e) { /* H5 无振动 */ }
    // 会话内核销记录（后端暂无教练核销历史接口）
    records.value.unshift({
      bizType: r.bizType,
      userName: r.userName,
      userPhone: r.userPhone,
      code: r.code || code,
      timeText: fmtTime(r.verifiedAt) || fmtTime(new Date()),
      already: !!r.alreadyVerified
    })
    if (records.value.length > 20) records.value.pop()
  } catch (e) {
    lastResult.value = null
    lastError.value = '核销失败：' + (e && e.message ? e.message : '码无效')
  } finally {
    verifying.value = false
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

.sk-gap {
  margin-bottom: 12px;
}

// 手动输码
.manual-card {
  margin-top: 16px;
  padding: 14px 16px;
}
.manual-tip {
  display: block;
  margin-bottom: 10px;
  @include md-body-small;
  color: var(--md-on-surface-variant);
}
.manual-row {
  display: flex;
  align-items: flex-end;
  gap: 12px;
}
.manual-input {
  flex: 1;
  min-width: 0;
}

// 结果反馈卡
.result-card {
  margin-top: 16px;
  padding: 16px;
  border-radius: $md-shape-md;
  &.is-ok {
    background: var(--md-primary-container);
    .result-icon,
    .result-title {
      color: var(--md-on-primary-container);
    }
    .result-line,
    .result-sub {
      color: var(--md-on-primary-container);
      opacity: 0.8;
    }
  }
  &.is-bad {
    background: var(--md-error-container);
    .result-icon,
    .result-title {
      color: var(--md-on-error-container);
    }
  }
}
.result-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.result-icon {
  flex-shrink: 0;
}
.result-title {
  @include md-title-medium;
  font-weight: 600;
}
.result-line {
  display: block;
  margin-top: 8px;
  @include md-body-medium;
}
.result-sub {
  display: block;
  margin-top: 2px;
  @include md-body-small;
}

// 核销记录
.rec-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 20px 0 10px;
  @include md-title-medium;
  color: var(--md-on-surface);
  &__icon {
    color: var(--md-primary);
  }
}
.rec-card {
  margin-bottom: 8px;
}
.rec-inner {
  padding: 12px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
}
.rec-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.rec-name {
  @include md-body-medium;
  color: var(--md-on-surface);
}
.rec-meta {
  @include md-body-small;
  color: var(--md-on-surface-variant);
}
.rec-icon-ok {
  color: var(--md-primary);
  flex-shrink: 0;
}
.rec-icon-warn {
  color: var(--md-tertiary);
  flex-shrink: 0;
}
</style>
