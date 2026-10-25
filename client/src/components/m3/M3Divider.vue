<template>
  <view class="m3-divider" :class="{ 'is-inset': inset }" />
</template>

<script setup>
// 分隔线
defineProps({ inset: Boolean })
</script>

<style lang="scss" scoped>
.m3-divider {
  height: 1px;
  background: var(--md-outline-variant);
  &.is-inset {
    margin-left: 16px;
  }
}
</style>
