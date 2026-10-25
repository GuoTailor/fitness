<template>
  <view class="md-page">
    <M3TopAppBar title="评价课程" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 骨架屏 -->
      <template v-if="loading">
        <M3Card variant="filled" class="rv-skeleton">
          <view class="rv-skeleton-inner">
            <M3Skeleton variant="text" width="50%" height="14" />
            <M3Skeleton variant="rect" width="100%" height="96" />
          </view>
        </M3Card>
      </template>

      <template v-else>
        <!-- 选择待评价课程 -->
        <M3Card v-if="!current && pending.length" variant="filled" class="rv-card">
          <view class="rv-card-inner">
            <text class="rv-section-title">待评价课程（{{ pending.length }}）</text>
            <view
              v-for="p in pending"
              :key="p.verificationId"
              class="rv-pend"
              @click="onPick(p)"
            >
              <view class="rv-pend-info">
                <text class="rv-pend-name md-ellipsis">{{ p.projectName }}</text>
                <text class="rv-pend-sub md-ellipsis">{{ p.coachName }} · {{ p.typeText }} · {{ p.dateText }}</text>
              </view>
              <view class="rv-pend-go">
                <text>去评价</text>
                <M3Icon name="chevron_right" :size="16" />
              </view>
            </view>
          </view>
        </M3Card>

        <!-- 评价表单 -->
        <template v-if="current">
          <!-- 课程信息 -->
          <M3Card variant="filled" class="rv-card">
            <view class="rv-card-inner">
              <view class="rv-cur-name md-ellipsis-2">{{ current.projectName }}</view>
              <text class="rv-cur-sub">{{ current.coachName }} · {{ current.typeText }} · {{ current.dateText }}</text>
            </view>
          </M3Card>

          <!-- 评分 + 内容 + 图片 -->
          <M3Card variant="filled" class="rv-card">
            <view class="rv-card-inner">
              <view class="rv-rate-row">
                <text class="rv-rate-label">课程体验</text>
                <M3Stars v-model="pkgStars" :size="28" editable />
              </view>
              <view class="rv-rate-row">
                <text class="rv-rate-label">教练服务</text>
                <M3Stars v-model="coachStars" :size="28" editable />
              </view>
              <M3Divider inset />
              <M3TextField
                v-model="content"
                label="上课感受"
                placeholder="说说上课感受，帮助更多小伙伴（选填）"
                multiline
                :maxlength="500"
                label-bg="var(--md-surface-container-high)"
              />
              <!-- 图片：最多 3 张 -->
              <view class="rv-img-grid">
                <view v-for="(img, i) in images" :key="img" class="rv-img-item">
                  <image :src="img" mode="aspectFill" class="rv-img" @click="onPreview(i)" />
                  <view class="rv-img-del" @click.stop="onRemoveImage(i)">
                    <M3Icon name="close" :size="12" />
                  </view>
                </view>
                <view v-if="images.length < MAX_IMAGES" class="rv-img-add" @click="onChooseImage">
                  <M3Progress v-if="uploading" type="circular" :size="20" indeterminate />
                  <M3Icon v-else name="add_a_photo" :size="24" />
                  <text class="rv-img-add-text">{{ uploading ? '上传中' : MAX_IMAGES - images.length + '/3' }}</text>
                </view>
              </view>
            </view>
          </M3Card>

          <M3Button
            variant="filled"
            block
            class="rv-submit"
            :loading="submitting"
            @click="onSubmit"
          >提交评价</M3Button>
        </template>

        <!-- 空态 -->
        <M3EmptyState
          v-if="!current && !pending.length"
          :icon="vid ? 'task_alt' : 'event_available'"
          :title="vid ? '该课程已评价过' : '暂无待评价课程'"
          :description="vid ? '' : '完成上课并核销后就可以评价啦'"
        />
      </template>
    </view>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onShow, onPageScroll } from '@dcloudio/uni-app'
import { request, uploadImage } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
import { fmtDate } from '@/utils/format'

const auth = useAuthStore()
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

const MAX_IMAGES = 3
const vid = ref(0)

const pending = ref([]) // 待评价列表（无 verificationId 时展示选择）
const current = ref(null) // 正在评价的课程
const pkgStars = ref(5)
const coachStars = ref(5)
const content = ref('')
const images = ref([]) // 已上传图片 URL
const uploading = ref(false)
const submitting = ref(false)
const loading = ref(true)

onLoad((query) => {
  vid.value = Number(query && query.verificationId) || 0
})

onShow(() => {
  load()
})

async function load() {
  loading.value = true
  try {
    const items = (await request({ url: '/api/v1/reviews/reviewable' })) || []
    const list = items.map((x) => ({
      ...x,
      dateText: fmtDate(x.verifiedAt),
      typeText: x.bizType === 'group' ? '团课' : '私教'
    }))
    if (vid.value) {
      const cur = list.find((x) => x.verificationId === vid.value)
      current.value = cur || null
      pending.value = []
    } else {
      pending.value = list
      current.value = list.length === 1 ? list[0] : null
    }
  } catch (e) {
    // 错误已由请求层 toast
  } finally {
    loading.value = false
  }
}

function onPick(item) {
  current.value = item
}

// 选图（最多 3 张）→ 逐张上传
function onChooseImage() {
  const left = MAX_IMAGES - images.value.length
  if (left <= 0 || uploading.value) return
  uni.chooseImage({
    count: left,
    sizeType: ['compressed'],
    success: async (res) => {
      const files = (res && res.tempFiles) || []
      if (!files.length) return
      uploading.value = true
      try {
        for (const f of files) {
          if (f.size > 2 * 1024 * 1024) {
            uni.showToast({ title: '单张图片不超过 2MB', icon: 'none' })
            continue
          }
          const path = f.path || f.tempFilePath
          if (!path) continue
          const data = await uploadImage(path)
          if (data && data.url) {
            images.value = images.value.concat(data.url).slice(0, MAX_IMAGES)
          }
        }
      } catch (e) {
        // uploadImage 已 toast
      } finally {
        uploading.value = false
      }
    }
  })
}

function onRemoveImage(i) {
  const arr = images.value.slice()
  arr.splice(i, 1)
  images.value = arr
}

function onPreview(i) {
  uni.previewImage({ current: images.value[i], urls: images.value })
}

async function onSubmit() {
  if (submitting.value || !current.value) return
  submitting.value = true
  try {
    await request({
      url: '/api/v1/reviews',
      method: 'POST',
      data: {
        verificationId: current.value.verificationId,
        packageRating: pkgStars.value,
        coachRating: coachStars.value,
        content: content.value,
        images: images.value
      }
    })
    uni.showToast({ title: '评价成功，感谢反馈', icon: 'success' })
    setTimeout(() => {
      uni.navigateBack({ fail: () => uni.reLaunch({ url: '/pages/mine/mine' }) })
    }, 800)
  } catch (e) {
    // request 已 toast
  } finally {
    submitting.value = false
  }
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

.rv-skeleton {
  margin-bottom: 12px;
}
.rv-skeleton-inner {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.rv-card {
  margin-bottom: 12px;
}
.rv-card-inner {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.rv-section-title {
  @include md-title-small;
  color: var(--md-on-surface);
}

// 待评价列表
.rv-pend {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  margin: -4px -8px 0;
  border-radius: $md-shape-md;
  transition: background $md-motion-short $md-motion-standard;
  &:active {
    background: var(--md-surface-container-highest);
  }
  &-info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  &-name {
    color: var(--md-on-surface);
    @include md-title-small;
  }
  &-sub {
    color: var(--md-on-surface-variant);
    @include md-body-small;
  }
  &-go {
    flex-shrink: 0;
    display: inline-flex;
    align-items: center;
    gap: 2px;
    color: var(--md-primary);
    @include md-label-medium;
  }
}

// 课程信息
.rv-cur-name {
  color: var(--md-on-surface);
  @include md-title-medium;
  line-height: 24px;
}
.rv-cur-sub {
  color: var(--md-on-surface-variant);
  @include md-body-small;
}

// 评分
.rv-rate-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.rv-rate-label {
  color: var(--md-on-surface);
  @include md-body-large;
}

// 图片九宫格
.rv-img-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.rv-img-item {
  position: relative;
  width: 88px;
  height: 88px;
}
.rv-img {
  width: 100%;
  height: 100%;
  border-radius: $md-shape-sm;
  background: var(--md-surface-container-highest);
}
.rv-img-del {
  position: absolute;
  top: -6px;
  right: -6px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--md-on-surface);
  color: var(--md-surface);
  display: flex;
  align-items: center;
  justify-content: center;
}
.rv-img-add {
  width: 88px;
  height: 88px;
  border-radius: $md-shape-sm;
  border: 1px dashed var(--md-outline-variant);
  color: var(--md-outline);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  cursor: pointer;
  transition: background $md-motion-short $md-motion-standard;
  &:active {
    background: var(--md-surface-container-highest);
  }
}
.rv-img-add-text {
  @include md-label-small;
}

.rv-submit {
  margin-top: 4px;
}
</style>
