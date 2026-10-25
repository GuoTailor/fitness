<template>
  <view class="m3-badge-wrap">
    <slot />
    <view v-if="count > 0 || dot" class="m3-badge" :class="{ 'is-dot': dot }">
      <text v-if="!dot" class="m3-badge__num">{{ count > max ? max + '+' : count }}</text>
    </view>
  </view>
</template>

<script setup>
// 徽标：数字角标 / 小红点
defineProps({
  count: { type: Number, default: 0 },
  max: { type: Number, default: 99 },
  dot: Boolean
})
</script>

<style lang="scss" scoped>
.m3-badge-wrap {
  position: relative;
  display: inline-flex;
}
.m3-badge {
  position: absolute;
  top: -4px;
  right: -8px;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border-radius: $md-shape-full;
  background: var(--md-error);
  display: flex;
  align-items: center;
  justify-content: center;
  &.is-dot {
    width: 6px;
    height: 6px;
    min-width: 0;
    padding: 0;
    top: 0;
    right: -2px;
  }
  &__num {
    color: var(--md-on-error);
    @include md-label-small;
  }
}
</style>
