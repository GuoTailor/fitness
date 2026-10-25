<template>
  <view class="m3-card" :class="['m3-card--' + variant, { 'is-clickable': clickable }]" @click="clickable && $emit('click')">
    <slot />
  </view>
</template>

<script setup>
// MD3 卡片三变体：elevated / filled / outlined
defineProps({
  variant: { type: String, default: 'filled' }, // elevated | filled | outlined
  clickable: Boolean
})
defineEmits(['click'])
</script>

<style lang="scss" scoped>
.m3-card {
  border-radius: $md-shape-md;
  overflow: hidden;
  transition: box-shadow $md-motion-short $md-motion-standard, transform $md-motion-short;
  &--elevated {
    background: var(--md-surface-container-low);
    @include md-elevation-1;
  }
  &--filled {
    background: var(--md-surface-container-high);
  }
  &--outlined {
    background: var(--md-surface);
    border: 1px solid var(--md-outline-variant);
  }
  &.is-clickable {
    cursor: pointer;
  }
  &.is-clickable:active {
    transform: scale(0.99);
  }
}
</style>
