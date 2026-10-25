<template>
  <view class="md-page">
    <M3TopAppBar title="留言答疑" :back="true" :elevated="scrolled" />

    <!-- 身份门槛 -->
    <view v-if="!checked" class="md-shell page-body">
      <M3Card v-for="i in 3" :key="i" variant="filled" class="sk-card">
        <view class="sk-inner">
          <M3Skeleton variant="text" width="35%" height="14" />
          <M3Skeleton variant="text" width="80%" height="14" />
        </view>
      </M3Card>
    </view>

    <M3EmptyState
      v-else-if="!auth.isCoach"
      icon="manage_accounts"
      title="需要教练身份"
      description="登录教练账号后即可答疑"
    >
      <M3Button variant="filled" icon="how_to_reg" @click="goApply">去入驻</M3Button>
    </M3EmptyState>

    <view v-else class="md-shell page-body">
      <!-- 说明 -->
      <view class="tip">
        <M3Icon name="info" :size="18" class="tip__icon" />
        <text class="tip__text">课包详情页的学员提问：回复带认证标识优先展示，优质问答可置顶沉淀为常见问题</text>
      </view>

      <!-- 回复编辑器 -->
      <M3Card v-if="editing" variant="filled" class="editor">
        <view class="editor-head">
          <text class="editor-title">回复提问</text>
        </view>
        <M3TextField
          v-model="content"
          label="回复内容"
          placeholder="回复提问（1000 字内）"
          :multiline="true"
          :auto-height="true"
          :maxlength="1000"
          class="editor-field"
        />
        <view class="editor-btns">
          <M3Button variant="text" @click="cancelEdit">取消</M3Button>
          <M3Button variant="filled" icon="send" :loading="saving" @click="onSave">发送回复</M3Button>
        </view>
      </M3Card>

      <!-- 列表骨架 -->
      <template v-if="loading && !list.length">
        <M3Card v-for="i in 3" :key="i" variant="filled" class="sk-card">
          <view class="sk-inner">
            <M3Skeleton variant="text" width="35%" height="14" />
            <M3Skeleton variant="text" width="80%" height="14" />
          </view>
        </M3Card>
      </template>

      <!-- 提问列表 -->
      <template v-else-if="list.length">
        <M3Card v-for="q in list" :key="q.id" variant="filled" class="q-card">
          <view class="q-inner">
            <view class="q-head">
              <text class="q-asker md-ellipsis">{{ q.asker || '学员' }}</text>
              <view class="q-ops">
                <M3Chip
                  :label="q.pinned ? '已置顶' : '置顶'"
                  :selected="!!q.pinned"
                  filter
                  @click="onPin(q)"
                />
                <M3Button variant="text" size="sm" @click="onReply(q)">回复</M3Button>
              </view>
            </view>
            <text class="q-content">{{ q.content }}</text>
            <text class="q-meta">{{ q.timeText }}</text>

            <view v-for="a in q.answers || []" :key="a.id" class="a-row">
              <text class="a-badge" :class="{ 'is-coach': a.isCoach }">{{ a.isCoach ? '教练' : '学员' }}</text>
              <view class="a-body">
                <text class="a-content">{{ a.content }}</text>
                <text class="a-meta">{{ a.authorName || '学员' }} · {{ a.timeText }}</text>
              </view>
            </view>
          </view>
        </M3Card>
      </template>

      <!-- 空态 -->
      <M3EmptyState
        v-else
        icon="forum"
        title="暂无学员提问"
        description="学员在课包详情页的提问会出现在这里"
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

// 回复编辑器
const editing = ref(0) // question id
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
    const rows = (await request({ url: '/api/v1/coach/qa' })) || []
    rows.forEach((q) => {
      q.timeText = fmtTime(q.createdAt).slice(0, 16)
      ;(q.answers || []).forEach((a) => {
        a.timeText = fmtTime(a.createdAt).slice(0, 16)
      })
    })
    list.value = rows
  } catch (e) {
    // toast 已由请求层处理
  } finally {
    loading.value = false
  }
}

function onReply(q) {
  editing.value = q.id
  content.value = ''
  uni.pageScrollTo({ scrollTop: 0, duration: 200 })
}

function cancelEdit() {
  editing.value = 0
  content.value = ''
}

async function onSave() {
  if (!content.value.trim()) {
    uni.showToast({ title: '请填写回复', icon: 'none' })
    return
  }
  if (saving.value) return
  saving.value = true
  try {
    await request({
      url: `/api/v1/coach/qa/${editing.value}/answers`,
      method: 'POST',
      data: { content: content.value.trim() }
    })
    editing.value = 0
    content.value = ''
    uni.showToast({ title: '已回复', icon: 'success' })
    load()
  } catch (e) {
    // toast 已由请求层处理
  } finally {
    saving.value = false
  }
}

// 置顶/取消置顶（沉淀为常见问题）
async function onPin(q) {
  try {
    await request({ url: `/api/v1/coach/qa/${q.id}/pin`, method: 'POST' })
    uni.showToast({ title: q.pinned ? '已取消置顶' : '已置顶', icon: 'none' })
    load()
  } catch (e) {
    // toast 已由请求层处理
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
  margin-bottom: 10px;
}
.editor-title {
  @include md-title-medium;
  font-weight: 600;
  color: var(--md-on-surface);
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

// 提问卡
.q-card {
  margin-bottom: 12px;
}
.q-inner {
  padding: 14px 16px;
}
.q-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.q-asker {
  flex: 1;
  min-width: 0;
  @include md-title-small;
  font-weight: 600;
  color: var(--md-on-surface);
}
.q-ops {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
.q-content {
  display: block;
  margin-top: 8px;
  @include md-body-medium;
  color: var(--md-on-surface);
  line-height: 22px;
}
.q-meta {
  display: block;
  margin-top: 4px;
  @include md-label-small;
  color: var(--md-on-surface-variant);
}

// 回答
.a-row {
  margin-top: 10px;
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 12px;
  border-radius: $md-shape-md;
  background: var(--md-surface-container-high);
}
.a-badge {
  flex-shrink: 0;
  padding: 2px 8px;
  border-radius: $md-shape-full;
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface-variant);
  @include md-label-small;
  &.is-coach {
    background: var(--md-primary-container);
    color: var(--md-on-primary-container);
  }
}
.a-body {
  flex: 1;
  min-width: 0;
}
.a-content {
  display: block;
  @include md-body-small;
  color: var(--md-on-surface);
  line-height: 20px;
}
.a-meta {
  display: block;
  margin-top: 4px;
  @include md-label-small;
  color: var(--md-on-surface-variant);
}
</style>
