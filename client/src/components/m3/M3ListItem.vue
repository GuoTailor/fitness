<template>
  <view class="m3-li" :class="{ 'is-clickable': clickable, 'is-disabled': disabled }" @click="clickable && $emit('click')">
    <view v-if="$slots.leading" class="m3-li__leading">
      <slot name="leading" />
    </view>
    <view class="m3-li__text">
      <text class="m3-li__headline md-ellipsis" :class="{ 'is-inline': headlineInline }">{{ headline }}</text>
      <text v-if="supporting" class="m3-li__supporting md-ellipsis">{{ supporting }}</text>
      <slot />
    </view>
    <view v-if="$slots.trailing || trailingIcon" class="m3-li__trailing">
      <slot name="trailing">
        <M3Icon :name="trailingIcon" :size="24" />
      </slot>
    </view>
  </view>
</template>

<script setup>
// MD3 列表项：headline 单行模式 + supporting 副文本；默认插槽可放自定义内容块
defineProps({
  headline: { type: String, default: '' },
  headlineInline: { type: Boolean, default: true },
  supporting: { type: String, default: '' },
  trailingIcon: { type: String, default: '' },
  clickable: Boolean,
  disabled: Boolean
})
defineEmits(['click'])
</script>

<style lang="scss" scoped>
.m3-li {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 12px 16px;
  min-height: 56px;
  cursor: pointer;
  &.is-clickable:active {
    background: rgba(0, 0, 0, $md-state-pressed-opacity);
  }
  &.is-disabled {
    opacity: 0.38;
    pointer-events: none;
  }
  &__leading {
    display: flex;
    align-items: center;
    color: var(--md-on-surface-variant);
    flex-shrink: 0;
  }
  &__text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  &__headline {
    color: var(--md-on-surface);
    @include md-body-large;
    &.is-inline {
      @include md-title-medium;
    }
  }
  &__supporting {
    color: var(--md-on-surface-variant);
    @include md-body-medium;
  }
  &__trailing {
    display: flex;
    align-items: center;
    color: var(--md-on-surface-variant);
    flex-shrink: 0;
  }
}
</style>
