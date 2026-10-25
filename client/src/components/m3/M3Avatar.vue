<template>
  <image
    v-if="src"
    class="m3-avatar"
    :src="src"
    :style="{ width: size + 'px', height: size + 'px' }"
    mode="aspectFill"
    @error="failed = true"
  />
  <view
    v-else
    class="m3-avatar m3-avatar--fallback"
    :style="{ width: size + 'px', height: size + 'px', fontSize: size * 0.42 + 'px' }"
  >
    <text>{{ initial }}</text>
  </view>
</template>

<script setup>
// 头像：无图或加载失败时回退到首字圆形占位
import { computed, ref, watch } from 'vue'

const props = defineProps({
  src: { type: String, default: '' },
  name: { type: String, default: '' },
  size: { type: Number, default: 40 }
})
const failed = ref(false)
watch(() => props.src, () => { failed.value = false })
const initial = computed(() => (props.name || '访').trim().charAt(0))
const showImg = computed(() => props.src && !failed.value)
defineExpose({ showImg })
</script>

<style lang="scss" scoped>
.m3-avatar {
  border-radius: 50%;
  flex-shrink: 0;
  display: block;
  &--fallback {
    background: var(--md-primary-container);
    color: var(--md-on-primary-container);
    display: flex;
    align-items: center;
    justify-content: center;
  }
}
</style>
