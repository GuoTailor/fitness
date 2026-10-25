<template>
  <view class="m3-seg">
    <view
      v-for="(opt, i) in options"
      :key="opt.value"
      class="m3-seg__item"
      :class="{
        'is-selected': modelValue === opt.value,
        'is-first': i === 0,
        'is-last': i === options.length - 1
      }"
      @click="$emit('update:modelValue', opt.value)"
    >
      <M3Icon v-if="opt.icon" :name="opt.icon" :size="18" />
      <text class="m3-seg__label">{{ opt.label }}</text>
    </view>
  </view>
</template>

<script setup>
// MD3 分段按钮（单选）
defineProps({
  options: { type: Array, default: () => [] }, // [{value, label, icon}]
  modelValue: { type: [String, Number], default: '' }
})
defineEmits(['update:modelValue'])
</script>

<style lang="scss" scoped>
.m3-seg {
  display: inline-flex;
  border: 1px solid var(--md-outline);
  border-radius: $md-shape-full;
  overflow: hidden;
  &__item {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    height: 40px;
    padding: 0 16px;
    border-left: 1px solid var(--md-outline);
    color: var(--md-on-surface);
    cursor: pointer;
    transition: background $md-motion-short;
    &.is-first {
      border-left: none;
    }
    &.is-selected {
      background: var(--md-secondary-container);
      color: var(--md-on-secondary-container);
    }
  }
  &__label {
    @include md-label-large;
    white-space: nowrap;
  }
}
</style>
