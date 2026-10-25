<template>
  <view class="m3-ib" :class="{ 'is-disabled': disabled }" @click="onClick">
    <M3Icon :name="icon" :size="size" />
  </view>
</template>

<script setup>
const props = defineProps({
  icon: { type: String, required: true },
  size: { type: Number, default: 24 },
  disabled: Boolean
})
const emit = defineEmits(['click'])
function onClick(e) {
  if (props.disabled) return
  emit('click', e)
}
</script>

<style lang="scss" scoped>
.m3-ib {
  position: relative;
  width: 40px;
  height: 40px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: $md-shape-full;
  color: var(--md-on-surface-variant);
  cursor: pointer;
  overflow: hidden;
  flex-shrink: 0;
  &::after {
    content: '';
    position: absolute;
    inset: 0;
    background: currentColor;
    opacity: 0;
    transition: opacity $md-motion-short;
  }
  &:active::after {
    opacity: $md-state-pressed-opacity;
  }
  &.is-disabled {
    opacity: 0.38;
    pointer-events: none;
  }
}
</style>
