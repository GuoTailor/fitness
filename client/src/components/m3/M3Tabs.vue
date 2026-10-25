<template>
  <view class="m3-tabs">
    <view
      v-for="tab in tabs"
      :key="tab.value"
      class="m3-tabs__tab"
      :class="{ 'is-active': tab.value === modelValue }"
      @click="$emit('update:modelValue', tab.value)"
    >
      <text class="m3-tabs__label">{{ tab.label }}</text>
      <view class="m3-tabs__indicator" />
    </view>
  </view>
</template>

<script setup>
// MD3 主标签页：底部指示条滑动跟随
defineProps({
  tabs: { type: Array, default: () => [] }, // [{value, label}]
  modelValue: { type: [String, Number], default: '' }
})
defineEmits(['update:modelValue'])
</script>

<style lang="scss" scoped>
.m3-tabs {
  display: flex;
  background: var(--md-surface);
  &__tab {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 12px 0 0;
    cursor: pointer;
    color: var(--md-on-surface-variant);
  }
  &__label {
    @include md-title-small;
    padding-bottom: 10px;
  }
  &__indicator {
    width: 100%;
    height: 3px;
    border-radius: $md-shape-full $md-shape-full 0 0;
    background: transparent;
    transition: background $md-motion-short;
  }
  &__tab.is-active {
    color: var(--md-primary);
    .m3-tabs__indicator {
      background: var(--md-primary);
    }
  }
}
</style>
