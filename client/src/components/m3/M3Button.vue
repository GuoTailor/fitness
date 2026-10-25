<template>
  <view
    class="m3-btn"
    :class="[
      'm3-btn--' + variant,
      'm3-btn--' + size,
      { 'is-block': block, 'is-disabled': disabled || loading }
    ]"
    @click="onClick"
  >
    <text v-if="loading" class="m3-btn__spinner" />
    <M3Icon v-else-if="icon" :name="icon" :size="size === 'sm' ? 16 : 18" class="m3-btn__icon" />
    <text class="m3-btn__label"><slot /></text>
  </view>
</template>

<script setup>
// MD3 按钮五变体：filled / tonal / outlined / text / elevated；danger=true 走错误色
const props = defineProps({
  variant: { type: String, default: 'filled' }, // filled | tonal | outlined | text | elevated
  size: { type: String, default: 'md' }, // sm(32px) | md(40px) | lg(48px)
  icon: { type: String, default: '' },
  block: Boolean,
  disabled: Boolean,
  loading: Boolean,
  danger: Boolean
})
const emit = defineEmits(['click'])

function onClick(e) {
  if (props.disabled || props.loading) return
  emit('click', e)
}
</script>

<style lang="scss" scoped>
.m3-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border-radius: $md-shape-full;
  overflow: hidden;
  cursor: pointer;
  transition: box-shadow $md-motion-short $md-motion-standard, opacity $md-motion-short;
  &::after {
    content: '';
    position: absolute;
    inset: 0;
    background: currentColor;
    opacity: 0;
    transition: opacity $md-motion-short;
  }
  &:active::after {
    opacity: $md-state-pressed-opacity;
  }
  &--sm {
    height: 32px;
    padding: 0 16px;
    .m3-btn__label { @include md-label-medium; }
  }
  &--md {
    height: 40px;
    padding: 0 24px;
    .m3-btn__label { @include md-label-large; }
  }
  &--lg {
    height: 48px;
    padding: 0 28px;
    .m3-btn__label { @include md-label-large; }
  }
  &--filled {
    background: var(--md-primary);
    color: var(--md-on-primary);
  }
  &--tonal {
    background: var(--md-secondary-container);
    color: var(--md-on-secondary-container);
  }
  &--elevated {
    background: var(--md-surface-container-lowest);
    color: var(--md-primary);
    @include md-elevation-1;
  }
  &--outlined {
    border: 1px solid var(--md-outline);
    color: var(--md-primary);
    background: transparent;
  }
  &--text {
    color: var(--md-primary);
    padding: 0 12px;
    background: transparent;
  }
  &.is-danger {
    &.m3-btn--filled {
      background: var(--md-error);
      color: var(--md-on-error);
    }
    &.m3-btn--tonal {
      background: var(--md-error-container);
      color: var(--md-on-error-container);
    }
    color: var(--md-error);
  }
  &.is-block {
    display: flex;
    width: 100%;
  }
  &.is-disabled {
    opacity: 0.38;
    pointer-events: none;
  }
  &__icon {
    flex-shrink: 0;
  }
  &__label {
    white-space: nowrap;
  }
  &__spinner {
    width: 16px;
    height: 16px;
    border: 2px solid currentColor;
    border-right-color: transparent;
    border-radius: 50%;
    animation: m3-spin 0.8s linear infinite;
  }
}
@keyframes m3-spin {
  to { transform: rotate(360deg); }
}
</style>
