<template>
  <view
    class="m3-switch"
    :class="{ 'is-on': modelValue, 'is-disabled': disabled }"
    @click="onTap"
  >
    <view class="m3-switch__track">
      <view class="m3-switch__thumb" />
    </view>
  </view>
</template>

<script setup>
// MD3 开关
const props = defineProps({
  modelValue: Boolean,
  disabled: Boolean
})
const emit = defineEmits(['update:modelValue', 'change'])

function onTap() {
  if (props.disabled) return
  emit('update:modelValue', !props.modelValue)
  emit('change', !props.modelValue)
}
</script>

<style lang="scss" scoped>
.m3-switch {
  display: inline-block;
  cursor: pointer;
  &__track {
    width: 52px;
    height: 32px;
    border-radius: $md-shape-full;
    background: var(--md-surface-container-highest);
    border: 2px solid var(--md-outline);
    display: flex;
    align-items: center;
    padding: 0 2px;
    transition: background $md-motion-medium, border-color $md-motion-medium;
  }
  &__thumb {
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: var(--md-outline);
    transition: all $md-motion-medium $md-motion-emphasized;
  }
  &.is-on {
    .m3-switch__track {
      background: var(--md-primary);
      border-color: var(--md-primary);
    }
    .m3-switch__thumb {
      width: 24px;
      height: 24px;
      background: var(--md-on-primary);
      transform: translateX(20px);
    }
  }
  &.is-disabled {
    opacity: 0.38;
    pointer-events: none;
  }
}
</style>
