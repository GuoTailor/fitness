<template>
  <view class="m3-empty">
    <M3Icon :name="icon" :size="56" class="m3-empty__icon" />
    <text class="m3-empty__title">{{ title }}</text>
    <text v-if="description" class="m3-empty__desc">{{ description }}</text>
    <view v-if="$slots.default" class="m3-empty__action">
      <slot />
    </view>
  </view>
</template>

<script setup>
// 空状态：图标 + 标题 + 描述 + 动作插槽
defineProps({
  icon: { type: String, default: 'inbox' },
  title: { type: String, default: '暂无内容' },
  description: { type: String, default: '' }
})
</script>

<style lang="scss" scoped>
.m3-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 48px 24px;
  text-align: center;
  &__icon {
    color: var(--md-outline);
    margin-bottom: 16px;
  }
  &__title {
    color: var(--md-on-surface);
    @include md-title-medium;
  }
  &__desc {
    margin-top: 6px;
    color: var(--md-on-surface-variant);
    @include md-body-medium;
  }
  &__action {
    margin-top: 20px;
  }
}
</style>
