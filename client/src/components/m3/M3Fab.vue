<template>
  <view class="m3-fab" :class="{ 'is-extended': !!label, 'is-disabled': disabled }" @click="!disabled && $emit('click')">
    <M3Icon :name="icon" :size="24" />
    <text v-if="label" class="m3-fab__label">{{ label }}</text>
  </view>
</template>

<script setup>
// MD3 FAB（可选扩展态）
defineProps({
  icon: { type: String, default: 'add' },
  label: { type: String, default: '' },
  disabled: Boolean
})
defineEmits(['click'])
</script>

<style lang="scss" scoped>
.m3-fab {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  width: 56px;
  height: 56px;
  border-radius: $md-shape-lg;
  background: var(--md-primary-container);
  color: var(--md-on-primary-container);
  cursor: pointer;
  @include md-elevation-2;
  transition: transform $md-motion-short, box-shadow $md-motion-short;
  &:active {
    transform: scale(0.96);
    @include md-elevation-3;
  }
  &.is-extended {
    width: auto;
    padding: 0 20px;
    height: 56px;
  }
  &.is-disabled {
    opacity: 0.38;
    pointer-events: none;
  }
  &__label {
    @include md-label-large;
    white-space: nowrap;
  }
}
</style>
