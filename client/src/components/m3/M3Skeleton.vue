<template>
  <view class="m3-skeleton" :class="'m3-skeleton--' + variant" :style="styleObj" />
</template>

<script setup>
// 骨架屏占位：text / rect / circle
import { computed } from 'vue'

const props = defineProps({
  variant: { type: String, default: 'rect' }, // text | rect | circle
  width: { type: [String, Number], default: '100%' },
  height: { type: [String, Number], default: 48 }
})
const styleObj = computed(() => {
  const w = typeof props.width === 'number' ? props.width + 'px' : props.width
  const h = typeof props.height === 'number' ? props.height + 'px' : props.height
  if (props.variant === 'circle') {
    const s = typeof props.width === 'number' ? props.width + 'px' : props.width
    return { width: s, height: s }
  }
  return { width: w, height: h }
})
</script>

<style lang="scss" scoped>
.m3-skeleton {
  display: block;
  border-radius: $md-shape-sm;
  background: linear-gradient(
    100deg,
    var(--md-surface-container) 40%,
    var(--md-surface-container-highest) 50%,
    var(--md-surface-container) 60%
  );
  background-size: 200% 100%;
  animation: m3-shimmer 1.4s linear infinite;
  &--text {
    height: 14px !important;
    border-radius: $md-shape-xs;
  }
  &--circle {
    border-radius: 50%;
  }
}
@keyframes m3-shimmer {
  from { background-position: 100% 0; }
  to { background-position: -100% 0; }
}
</style>
