<template>
  <view class="md-page">
    <M3TopAppBar :title="title" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 骨架 -->
      <template v-if="loading">
        <view class="sk-lines">
          <M3Skeleton v-for="i in 6" :key="i" variant="text" :height="14" :width="i === 6 ? '55%' : '100%'" />
        </view>
      </template>

      <!-- 正文（纯文本预格式化排版） -->
      <view v-else class="ag-content">{{ content }}</view>
    </view>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad, onPageScroll } from '@dcloudio/uni-app'
import { request } from '@/utils/request'

const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

const title = ref('协议')
const content = ref('')
const loading = ref(true)

onLoad(async (query) => {
  // 公开接口，无需登录
  const type = query && query.type === 'user' ? 'user' : 'privacy'
  try {
    const data = await request({ url: '/api/v1/agreement', data: { type } })
    title.value = (data && data.title) || '协议'
    content.value = (data && data.content) || ''
  } catch (e) {
    // 错误已由请求层 toast
  } finally {
    loading.value = false
  }
})
</script>

<style lang="scss" scoped>
.page-body {
  padding: 16px 16px 32px;
}

.sk-lines {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding-top: 8px;
}

.ag-content {
  white-space: pre-wrap;
  word-break: break-word;
  @include md-body-large;
  color: var(--md-on-surface);
  line-height: 26px;
}
</style>
