<template>
  <view class="md-page">
    <M3TopAppBar title="学员反馈" :back="true" :elevated="scrolled" />

    <!-- 身份门槛 -->
    <view v-if="!checked" class="md-shell page-body">
      <M3Card v-for="i in 3" :key="i" variant="filled" class="sk-card">
        <view class="sk-inner">
          <M3Skeleton variant="text" width="40%" height="14" />
          <M3Skeleton variant="text" width="75%" height="14" />
        </view>
      </M3Card>
    </view>

    <M3EmptyState
      v-else-if="!auth.isCoach"
      icon="manage_accounts"
      title="需要教练身份"
      description="登录教练账号后即可填写学员反馈"
    >
      <M3Button variant="filled" icon="how_to_reg" @click="goApply">去入驻</M3Button>
    </M3EmptyState>

    <view v-else class="md-shell page-body">
      <!-- 说明 -->
      <view class="tip">
        <M3Icon name="info" :size="18" class="tip__icon" />
        <text class="tip__text">对已核销课程填写训练反馈（动作要点/下次计划），学员在「训练跟踪」中可见</text>
      </view>

      <!-- 反馈编辑器 -->
      <M3Card v-if="editing" variant="filled" class="editor">
        <view class="editor-head">
          <text class="editor-title">填写反馈</text>
          <text class="editor-sub">{{ editingName }}</text>
        </view>
        <M3TextField
          v-model="content"
          label="训练反馈"
          placeholder="如：深蹲动作标准，下次加强核心与髋关节灵活性…（1000 字内）"
          :multiline="true"
          :auto-height="true"
          :maxlength="1000"
          class="editor-field"
        />
        <view class="editor-btns">
          <M3Button variant="text" @click="cancelEdit">取消</M3Button>
          <M3Button variant="filled" icon="send" :loading="saving" @click="onSave">保存反馈</M3Button>
        </view>
      </M3Card>

      <!-- 列表骨架 -->
      <template v-if="loading && !list.length">
        <M3Card v-for="i in 3" :key="i" variant="filled" class="sk-card">
          <view class="sk-inner">
            <M3Skeleton variant="text" width="40%" height="14" />
            <M3Skeleton variant="text" width="75%" height="14" />
          </view>
        </M3Card>
      </template>

      <!-- 已核销课程列表 -->
      <template v-else-if="list.length">
        <M3Card v-for="x in list" :key="x.verificationId" variant="filled" class="fb-card">
          <view class="fb-inner">
            <view class="fb-head">
              <text class="fb-name md-ellipsis">{{ x.userName || '学员 #' + x.verificationId }}</text>
              <text class="fb-type">{{ x.typeText }}</text>
            </view>
            <text class="fb-date">{{ x.dateText }}</text>
            <text v-if="x.written" class="fb-content">{{ x.feedback.content }}</text>
            <text v-else class="fb-content is-none">未填写</text>
            <view class="fb-ops" @click="onWrite(x)">
              <text class="fb-ops__text">{{ x.written ? '修订' : '填写反馈' }}</text>
              <M3Icon name="chevron_right" :size="18" class="fb-ops__icon" />
            </view>
          </view>
        </M3Card>
      </template>

      <!-- 空态 -->
      <M3EmptyState
        v-else
        icon="task_alt"
        title="暂无已核销课程"
        description="核销课程后可在这里填写训练反馈"
      />
    </view>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onShow, onPageScroll } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
import { fmtTime } from '@/utils/format'

const auth = useAuthStore()

const checked = ref(false)
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

const list = ref([])
const loading = ref(false)

// 编辑器
const editing = ref(0) // verificationId
const editingName = ref('')
const content = ref('')
const saving = ref(false)

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
    const rows = (await request({ url: '/api/v1/coach/feedbacks' })) || []
    rows.forEach((x) => {
      x.dateText = fmtTime(x.verifiedAt).slice(0, 16)
      x.typeText = x.bizType === 'group' ? '团课' : '私教'
      x.written = !!(x.feedback && x.feedback.content)
    })
    list.value = rows
  } catch (e) {
    // toast 已由请求层处理
  } finally {
    loading.value = false
  }
}

function onWrite(x) {
  editing.value = x.verificationId
  editingName.value = `${x.userName || '学员'} · ${x.typeText} · ${x.dateText}`
  content.value = (x.feedback && x.feedback.content) || ''
  // 编辑器固定在列表上方，滚动到顶便于输入
  uni.pageScrollTo({ scrollTop: 0, duration: 200 })
}

function cancelEdit() {
  editing.value = 0
  content.value = ''
}

async function onSave() {
  if (!content.value.trim()) {
    uni.showToast({ title: '请填写反馈内容', icon: 'none' })
    return
  }
  if (saving.value) return
  saving.value = true
  try {
    await request({
      url: '/api/v1/coach/feedback',
      method: 'POST',
      data: { verificationId: editing.value, content: content.value }
    })
    editing.value = 0
    content.value = ''
    uni.showToast({ title: '已保存', icon: 'success' })
    load()
  } catch (e) {
    // toast 已由请求层处理
  } finally {
    saving.value = false
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

// 编辑器
.editor {
  margin-bottom: 16px;
  padding: 14px 16px;
}
.editor-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}
.editor-title {
  @include md-title-medium;
  font-weight: 600;
  color: var(--md-on-surface);
}
.editor-sub {
  @include md-label-small;
  color: var(--md-on-surface-variant);
}
.editor-field {
  display: block;
}
.editor-btns {
  margin-top: 12px;
  display: flex;
  justify-content: flex-end;
  gap: 8px;
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

// 反馈卡
.fb-card {
  margin-bottom: 12px;
}
.fb-inner {
  padding: 14px 16px;
}
.fb-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.fb-name {
  flex: 1;
  min-width: 0;
  @include md-title-small;
  font-weight: 600;
  color: var(--md-on-surface);
}
.fb-type {
  flex-shrink: 0;
  padding: 2px 8px;
  border-radius: $md-shape-full;
  background: var(--md-tertiary-container);
  color: var(--md-on-tertiary-container);
  @include md-label-small;
}
.fb-date {
  display: block;
  margin-top: 4px;
  @include md-label-small;
  color: var(--md-on-surface-variant);
}
.fb-content {
  display: block;
  margin-top: 8px;
  @include md-body-medium;
  color: var(--md-on-surface);
  line-height: 22px;
  &.is-none {
    color: var(--md-outline);
  }
}
.fb-ops {
  margin-top: 10px;
  display: inline-flex;
  align-items: center;
  gap: 2px;
  cursor: pointer;
  &:active {
    opacity: 0.7;
  }
  &__text {
    @include md-label-large;
    color: var(--md-primary);
  }
  &__icon {
    color: var(--md-primary);
  }
}
</style>
