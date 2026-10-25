<template>
  <text class="md-icon m3-icon" :class="'md-icon-' + name" :style="{ fontSize: size + 'px', color: color }" />
</template>

<script setup>
// Material Symbols Rounded 图标（icons.scss 由 scripts/build_icon_font.mjs 生成）
defineProps({
  name: { type: String, required: true },
  size: { type: Number, default: 24 },
  color: { type: String, default: '' }
})
</script>

<style lang="scss" scoped>
.m3-icon {
  line-height: 1;
}
</style>
