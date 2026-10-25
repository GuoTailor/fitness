<template>
  <view class="md-page">
    <M3TopAppBar title="学员评价" :back="true" :elevated="scrolled" />

    <!-- 身份门槛 -->
    <view v-if="!checked" class="md-shell page-body">
      <M3Card v-for="i in 3" :key="i" variant="filled" class="sk-card">
        <view class="sk-inner">
          <view class="sk-row">
            <M3Skeleton variant="circle" width="36" height="36" />
            <M3Skeleton variant="text" width="40%" height="14" />
          </view>
          <M3Skeleton variant="text" width="85%" height="14" />
          <M3Skeleton variant="text" width="60%" height="14" />
        </view>
      </M3Card>
    </view>

    <M3EmptyState
      v-else-if="!auth.isCoach"
      icon="manage_accounts"
      title="需要教练身份"
      description="登录教练账号后即可查看学员评价"
    >
      <M3Button variant="filled" icon="how_to_reg" @click="goApply">去入驻</M3Button>
    </M3EmptyState>

    <view v-else class="md-shell page-body">
      <!-- 骨架 -->
      <template v-if="loading && !items.length">
        <M3Card v-for="i in 3" :key="i" variant="filled" class="sk-card">
          <view class="sk-inner">
            <view class="sk-row">
              <M3Skeleton variant="circle" width="36" height="36" />
              <M3Skeleton variant="text" width="40%" height="14" />
            </view>
            <M3Skeleton variant="text" width="85%" height="14" />
            <M3Skeleton variant="text" width="60%" height="14" />
          </view>
        </M3Card>
      </template>

      <!-- 评价列表 -->
      <template v-else-if="items.length">
        <M3Card v-for="(x, i) in items" :key="i" variant="filled" class="rv-card">
          <view class="rv-inner">
            <view class="rv-head">
              <M3Avatar :src="x.avatarUrl" :name="x.nickname" :size="36" />
              <view class="rv-head-main">
                <text class="rv-nick md-ellipsis">{{ x.nickname }}</text>
                <view class="rv-stars">
                  <M3Stars :model-value="x.review.rating" :size="14" show-value />
                </view>
              </view>
              <view class="st" :style="{ color: statusColor(x.review.status) }">
                <view class="st__dot" :style="{ background: statusColor(x.review.status) }" />
                <text class="st__label">{{ statusText(x.review.status) }}</text>
              </view>
            </view>

            <text class="rv-time">{{ x.timeText }}</text>
            <text v-if="x.review.content" class="rv-content">{{ x.review.content }}</text>

            <view v-if="x.images && x.images.length" class="rv-imgs">
              <image
                v-for="(img, j) in x.images"
                :key="j"
                class="rv-img"
                :src="img"
                mode="aspectFill"
                @click="onPreview(x.images, j)"
              />
            </view>

            <view v-if="x.review.coachReply" class="rv-reply">
              <text class="rv-reply-tag">我的回复</text>
              <text class="rv-reply-text">{{ x.review.coachReply }}</text>
            </view>

            <view v-if="!x.review.coachReply && x.review.status === 'approved'" class="rv-ops">
              <M3Button variant="outlined" size="sm" icon="chat_bubble" @click="openReply(x)">回复</M3Button>
            </view>
          </view>
        </M3Card>
      </template>

      <!-- 空态 -->
      <M3EmptyState
        v-else
        icon="reviews"
        title="暂无学员评价"
        description="学员上完课后可以对你进行评价"
      />
    </view>

    <!-- 回复评价 -->
    <M3Dialog :open="replyOpen" title="回复评价" icon="chat_bubble" :dismissible="false">
      <M3TextField
        v-model="replyContent"
        label="回复内容"
        placeholder="感谢您的反馈…（200 字内）"
        :multiline="true"
        :auto-height="true"
        :maxlength="200"
        class="reply-input"
      />
      <template #actions>
        <M3Button variant="text" @click="replyOpen = false">取消</M3Button>
        <M3Button variant="filled" :loading="replying" @click="doReply">发送</M3Button>
      </template>
    </M3Dialog>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onShow, onPageScroll } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
import { fmtDate } from '@/utils/format'

const auth = useAuthStore()

const checked = ref(false)
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

const STATUS = {
  pending: ['审核中', 'var(--md-tertiary)'],
  approved: ['已通过', 'var(--md-primary)'],
  hidden: ['已隐藏', 'var(--md-on-surface-variant)']
}
const statusText = (s) => (STATUS[s] || [s])[0]
const statusColor = (s) => (STATUS[s] || ['', 'var(--md-on-surface-variant)'])[1]

const items = ref([])
const loading = ref(false)

// 回复
const replyOpen = ref(false)
const replyId = ref(0)
const replyContent = ref('')
const replying = ref(false)

onLoad(async () => {
  await auth.ensureLogin().catch(() => {})
  if (auth.token && !auth.isCoach) await auth.refreshMe()
  checked.value = true
  if (auth.isCoach) load()
})

onShow(() => {
  if (checked.value && auth.isCoach) load()
})

async function load() {
  loading.value = true
  try {
    const data = await request({ url: '/api/v1/coach/reviews' })
    const rows = (data && data.items) || data || []
    rows.forEach((x) => {
      x.nickname = (x.nickname || '学员').slice(0, 12)
      x.timeText = fmtDate(x.review.createdAt)
      x.images = x.images || []
    })
    items.value = rows
  } catch (e) {
    // toast 已由请求层处理
  } finally {
    loading.value = false
  }
}

function onPreview(images, index) {
  uni.previewImage({ current: images[index], urls: images })
}

function openReply(x) {
  replyId.value = x.review.id
  replyContent.value = ''
  replyOpen.value = true
}

async function doReply() {
  const content = replyContent.value.trim()
  if (!content) {
    uni.showToast({ title: '请填写回复内容', icon: 'none' })
    return
  }
  if (replying.value) return
  replying.value = true
  try {
    await request({
      url: `/api/v1/coach/reviews/${replyId.value}/reply`,
      method: 'POST',
      data: { content }
    })
    replyOpen.value = false
    uni.showToast({ title: '已回复', icon: 'success' })
    load()
  } catch (e) {
    // toast 已由请求层处理
  } finally {
    replying.value = false
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
  margin-bottom: 12px;
}
.sk-inner {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.sk-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

// 评价卡
.rv-card {
  margin-bottom: 12px;
}
.rv-inner {
  padding: 14px 16px;
}
.rv-head {
  display: flex;
  align-items: center;
  gap: 10px;
}
.rv-head-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.rv-nick {
  @include md-title-small;
  color: var(--md-on-surface);
}
.rv-stars {
  display: flex;
}
.rv-time {
  display: block;
  margin-top: 8px;
  @include md-label-small;
  color: var(--md-on-surface-variant);
}
.rv-content {
  display: block;
  margin-top: 6px;
  @include md-body-medium;
  color: var(--md-on-surface);
  line-height: 22px;
}

// 评价图片
.rv-imgs {
  margin-top: 10px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.rv-img {
  width: calc((100% - 16px) / 3);
  height: 96px;
  border-radius: $md-shape-sm;
  background: var(--md-surface-container-highest);
}

// 教练回复
.rv-reply {
  margin-top: 10px;
  padding: 10px 12px;
  border-radius: $md-shape-md;
  background: var(--md-surface-container-high);
}
.rv-reply-tag {
  display: block;
  @include md-label-medium;
  color: var(--md-primary);
  margin-bottom: 4px;
}
.rv-reply-text {
  @include md-body-small;
  color: var(--md-on-surface-variant);
  line-height: 20px;
}
.rv-ops {
  margin-top: 12px;
  display: flex;
  justify-content: flex-end;
}

.reply-input {
  display: block;
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
</style>
