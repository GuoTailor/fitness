<template>
  <view
    class="m3-chip"
    :class="['m3-chip--' + variant, { 'is-selected': selected, 'is-disabled': disabled }]"
    @click="onClick"
  >
    <slot name="leading">
      <M3Icon v-if="selected && filter" name="check" :size="18" class="m3-chip__icon" />
    </slot>
    <text class="m3-chip__label">{{ label }}</text>
    <slot name="trailing">
      <M3Icon v-if="closable" name="close" :size="18" class="m3-chip__icon" @click="$emit('close')" />
    </slot>
  </view>
</template>

<script setup>
// MD3 Chip：筛选/辅助标签。filter=true 时选中态显示对勾
const props = defineProps({
  label: { type: String, default: '' },
  selected: Boolean,
  filter: Boolean,
  closable: Boolean,
  disabled: Boolean,
  variant: { type: String, default: 'outlined' } // outlined | tonal
})
const emit = defineEmits(['click', 'close'])

function onClick(e) {
  if (props.disabled) return
  emit('click', e)
}
</script>

<style lang="scss" scoped>
.m3-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 32px;
  padding: 0 12px;
  border-radius: $md-shape-sm;
  cursor: pointer;
  flex-shrink: 0;
  transition: background $md-motion-short, border-color $md-motion-short;
  &--outlined {
    border: 1px solid var(--md-outline-variant);
    color: var(--md-on-surface-variant);
  }
  &--tonal {
    background: var(--md-secondary-container);
    color: var(--md-on-secondary-container);
  }
  &.is-selected {
    background: var(--md-secondary-container);
    border-color: transparent;
    color: var(--md-on-secondary-container);
  }
  &.is-disabled {
    opacity: 0.38;
    pointer-events: none;
  }
  &__label {
    @include md-label-large;
    white-space: nowrap;
  }
  &__icon {
    color: inherit;
  }
}
</style>
