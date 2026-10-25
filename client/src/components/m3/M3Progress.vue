<template>
  <view v-if="type === 'linear'" class="m3-progress" :style="{ width: width }">
    <view class="m3-progress__bar" :class="{ 'is-indeterminate': indeterminate }">
      <view class="m3-progress__fill" :style="fillStyle" />
    </view>
  </view>
  <view v-else class="m3-circular" :style="{ width: size + 'px', height: size + 'px' }">
    <view class="m3-circular__ring" :class="{ 'is-indeterminate': indeterminate }" :style="ringStyle" />
  </view>
</template>

<script setup>
// 进度：linear（线性）/ circular（环形）；indeterminate 不定态
import { computed } from 'vue'

const props = defineProps({
  type: { type: String, default: 'linear' },
  modelValue: { type: Number, default: 0 }, // 0-100
  indeterminate: Boolean,
  width: { type: String, default: '100%' },
  size: { type: Number, default: 32 }
})
const fillStyle = computed(() =>
  props.indeterminate ? {} : { width: Math.min(100, Math.max(0, props.modelValue)) + '%' }
)
const ringStyle = computed(() => (props.indeterminate ? {} : {}))
</script>

<style lang="scss" scoped>
.m3-progress {
  &__bar {
    height: 4px;
    border-radius: $md-shape-full;
    background: var(--md-surface-container-highest);
    overflow: hidden;
  }
  &__fill {
    height: 100%;
    background: var(--md-primary);
    border-radius: $md-shape-full;
    transition: width $md-motion-medium $md-motion-standard;
  }
}
.m3-circular {
  display: inline-block;
  &__ring {
    width: 100%;
    height: 100%;
    border-radius: 50%;
    border: 3px solid var(--md-surface-container-highest);
    border-top-color: var(--md-primary);
    animation: m3-rotate 0.9s linear infinite;
    &.is-indeterminate {
      border-bottom-color: transparent;
    }
  }
}
@keyframes m3-rotate {
  to { transform: rotate(360deg); }
}
.m3-progress__bar.is-indeterminate .m3-progress__fill {
  width: 40% !important;
  animation: m3-slide 1.1s $md-motion-standard infinite;
}
@keyframes m3-slide {
  from { margin-left: -40%; }
  to { margin-left: 100%; }
}
</style>
