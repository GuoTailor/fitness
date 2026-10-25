<template>
  <view class="md-page">
    <M3TopAppBar title="教练入驻" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 我的申请状态 -->
      <view v-if="mine" class="st-banner" :class="'st-banner--' + mine.status">
        <M3Icon :name="stIcon" :size="22" class="st-banner-icon" />
        <view class="st-banner-body">
          <view class="st-banner-head">
            <text class="st-banner-name">{{ mine.statusText }}</text>
            <text class="st-banner-time">{{ mine.createdText }}</text>
          </view>
          <text v-if="mine.status === 2 && mine.rejectReason" class="st-banner-reason">
            驳回原因：{{ mine.rejectReason }}
          </text>
          <text v-if="mine.status === 0" class="st-banner-tip">
            审核通常在 1~3 个工作日内完成，结果将通过消息通知
          </text>
          <text v-if="mine.status === 1" class="st-banner-tip">
            恭喜！返回「我的」页即可进入教练工作台
          </text>
          <text v-if="mine.status === 2" class="st-banner-tip">可修改后重新提交申请</text>
        </view>
      </view>

      <!-- 骨架 -->
      <M3Card v-if="mineLoading && !mine" variant="filled" class="form-card sk-card">
        <M3Skeleton v-for="i in 5" :key="i" variant="rect" :height="52" />
      </M3Card>

      <!-- 申请表单（无申请或已被驳回时可提交） -->
      <M3Card v-if="!mineLoading && (!mine || mine.status === 2)" variant="filled" class="form-card">
        <view class="form-title-row">
          <M3Icon name="how_to_reg" :size="20" class="form-title-icon" />
          <text class="form-title">入驻申请</text>
        </view>

        <M3TextField
          v-model="form.name"
          label="姓名"
          placeholder="真实姓名"
          :maxlength="20"
          label-bg="var(--md-surface-container-high)"
        />
        <view class="form-field">
          <text class="form-label">性别</text>
          <M3SegmentedButton
            :options="[{ value: 1, label: '男' }, { value: 2, label: '女' }]"
            v-model="form.gender"
          />
        </view>
        <M3TextField
          v-model="form.years"
          label="从业年限"
          placeholder="如 5"
          type="number"
          :maxlength="3"
          label-bg="var(--md-surface-container-high)"
        />
        <M3TextField
          v-model="form.specialties"
          label="擅长方向"
          placeholder="逗号分隔，如 减脂,增肌,体态"
          :maxlength="60"
          label-bg="var(--md-surface-container-high)"
        />
        <M3TextField
          v-model="form.phone"
          label="联系方式"
          placeholder="手机号/微信（便于沟通）"
          :maxlength="30"
          label-bg="var(--md-surface-container-high)"
        />

        <!-- 资质证书 -->
        <view class="form-field">
          <text class="form-label">资质证书（1~6 张）</text>
          <view class="cert-grid">
            <view v-for="(img, i) in certImages" :key="img" class="cert-item">
              <image class="cert-img" :src="img" mode="aspectFill" @click="previewCert(img)" />
              <view class="cert-del" @click.stop="removeCert(i)">
                <M3Icon name="close" :size="13" />
              </view>
            </view>
            <view v-if="certImages.length < 6" class="cert-add" @click="addCert">
              <M3Icon name="add_a_photo" :size="26" />
              <text class="cert-add-text">{{ uploading ? '上传中…' : '添加' }}</text>
            </view>
          </view>
          <text v-if="certError" class="form-error">{{ certError }}</text>
        </view>

        <M3TextField
          v-model="form.intro"
          label="自我介绍"
          placeholder="从业经历、教学风格、擅长人群等（选填）"
          multiline
          :maxlength="1000"
          label-bg="var(--md-surface-container-high)"
        />

        <M3Button
          variant="filled"
          block
          icon="send"
          :loading="submitting"
          class="submit-btn"
          @click="onSubmit"
        >提交申请</M3Button>
      </M3Card>
    </view>
  </view>
</template>

<script setup>
import { ref, computed } from 'vue'
import { onLoad, onShow, onPageScroll } from '@dcloudio/uni-app'
import { request, uploadImage } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
import { fmtTime } from '@/utils/format'

const auth = useAuthStore()

const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

const STATUS_TEXT = { 0: '待审核', 1: '已通过', 2: '已驳回' }

const mine = ref(null)
const mineLoading = ref(true)
const form = ref({ name: '', gender: 0, years: '', specialties: '', intro: '', phone: '' })
const certImages = ref([])
const certError = ref('')
const uploading = ref(false)
const submitting = ref(false)

const stIcon = computed(() => {
  const s = mine.value && mine.value.status
  return s === 1 ? 'check_circle' : s === 2 ? 'cancel' : 'pending'
})

onLoad(async () => {
  await auth.ensureLogin().catch(() => {})
})

onShow(() => {
  if (!auth.token) return
  loadMine()
})

async function loadMine() {
  mineLoading.value = true
  try {
    const data = await request({ url: '/api/v1/coach-applications/mine' })
    if (data && data.exists) {
      const rec = data.application
      rec.statusText = STATUS_TEXT[rec.status] || ''
      rec.createdText = fmtTime(rec.createdAt)
      mine.value = rec
    } else {
      mine.value = null
    }
  } catch (e) {
    mine.value = null
  } finally {
    mineLoading.value = false
  }
}

// ---- 资质图：选图 → 逐张上传 ----
function addCert() {
  const left = 6 - certImages.value.length
  if (left <= 0) {
    uni.showToast({ title: '最多 6 张', icon: 'none' })
    return
  }
  uni.chooseImage({
    count: left,
    sizeType: ['compressed'],
    success: async (res) => {
      const paths = (res.tempFilePaths || []).slice(0, left)
      if (!paths.length) return
      certError.value = ''
      uploading.value = true
      try {
        for (const p of paths) {
          const r = await uploadImage(p)
          if (r && r.url) certImages.value.push(r.url)
        }
      } catch (e) {
        // toast 已处理
      } finally {
        uploading.value = false
      }
    }
  })
}

function removeCert(i) {
  certImages.value.splice(i, 1)
}

function previewCert(current) {
  uni.previewImage({ urls: certImages.value, current })
}

// ---- 提交 ----
async function onSubmit() {
  const f = form.value
  const name = (f.name || '').trim()
  if (name.length < 2) {
    uni.showToast({ title: '请填写真实姓名', icon: 'none' })
    return
  }
  if (!certImages.value.length) {
    certError.value = '请至少上传 1 张资质证书'
    return
  }
  submitting.value = true
  try {
    await request({
      url: '/api/v1/coach-applications',
      method: 'POST',
      data: {
        name,
        gender: f.gender,
        years: parseInt(f.years, 10) || 0,
        specialties: f.specialties,
        intro: f.intro,
        phone: f.phone,
        certImages: JSON.stringify(certImages.value)
      }
    })
    uni.showToast({ title: '已提交，等待审核', icon: 'success' })
    loadMine()
  } catch (e) {
    // toast 已处理
  } finally {
    submitting.value = false
  }
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

// ============ 申请状态横幅（语义色） ============
.st-banner {
  display: flex;
  gap: 12px;
  padding: 14px 16px;
  margin-bottom: 12px;
  border-radius: $md-shape-md;

  &--0 {
    background: var(--md-secondary-container);
    color: var(--md-on-secondary-container);
  }
  &--1 {
    background: var(--md-primary-container);
    color: var(--md-on-primary-container);
  }
  &--2 {
    background: var(--md-error-container);
    color: var(--md-on-error-container);
  }
}

.st-banner-icon {
  flex-shrink: 0;
}

.st-banner-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.st-banner-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}

.st-banner-name {
  @include md-title-medium;
  font-weight: 600;
}

.st-banner-time {
  @include md-label-small;
  opacity: 0.72;
}

.st-banner-reason {
  @include md-body-medium;
  line-height: 20px;
}

.st-banner-tip {
  @include md-body-small;
  opacity: 0.85;
  line-height: 18px;
}

// ============ 表单 ============
.form-card {
  padding: 20px 16px;
  margin-bottom: 12px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.sk-card {
  gap: 12px;
}

.form-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 2px;
}

.form-title-icon {
  color: var(--md-primary);
}

.form-title {
  @include md-title-medium;
  color: var(--md-on-surface);
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.form-label {
  @include md-body-medium;
  color: var(--md-on-surface-variant);
}

.form-error {
  @include md-body-small;
  color: var(--md-error);
}

// ---- 资质图 ----
.cert-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.cert-item {
  position: relative;
  width: 104px;
  height: 104px;
}

.cert-img {
  width: 100%;
  height: 100%;
  border-radius: $md-shape-sm;
  background: var(--md-surface-container-highest);

  &:active {
    opacity: 0.85;
  }
}

.cert-del {
  position: absolute;
  top: -6px;
  right: -6px;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--md-inverse-surface);
  color: var(--md-inverse-on-surface);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;

  &:active {
    opacity: 0.8;
  }
}

.cert-add {
  width: 104px;
  height: 104px;
  border: 1px dashed var(--md-outline);
  border-radius: $md-shape-sm;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: var(--md-on-surface-variant);
  cursor: pointer;
  box-sizing: border-box;

  &:active {
    background: rgba(0, 0, 0, $md-state-pressed-opacity);
  }
}

.cert-add-text {
  @include md-label-medium;
}

.submit-btn {
  margin-top: 4px;
}
</style>
