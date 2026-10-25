<template>
  <view v-if="open" class="m3-dialog-scrim" @click="onScrim">
    <view class="m3-dialog" @click.stop>
      <M3Icon v-if="icon" :name="icon" :size="24" class="m3-dialog__icon" />
      <text v-if="title" class="m3-dialog__title">{{ title }}</text>
      <scroll-view scroll-y class="m3-dialog__body">
        <slot />
      </scroll-view>
      <view class="m3-dialog__actions">
        <slot name="actions" />
      </view>
    </view>
  </view>
</template>

<script setup>
// MD3 基础对话框：scrim 点击关闭（dismissible=false 时仅按钮关闭）
const props = defineProps({
  open: Boolean,
  title: { type: String, default: '' },
  icon: { type: String, default: '' },
  dismissible: { type: Boolean, default: true }
})
const emit = defineEmits(['close'])

function onScrim() {
  if (props.dismissible) emit('close')
}
</script>

<style lang="scss" scoped>
.m3-dialog-scrim {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: var(--md-scrim);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  animation: m3-fade-in $md-motion-short both;
}
.m3-dialog {
  width: 100%;
  max-width: 360px;
  max-height: 80vh;
  background: var(--md-surface-container-high);
  border-radius: $md-shape-xl;
  padding: 24px;
  display: flex;
  flex-direction: column;
  animation: m3-dialog-in $md-motion-medium $md-motion-emphasized both;
  &__icon {
    color: var(--md-secondary);
    margin-bottom: 12px;
  }
  &__title {
    color: var(--md-on-surface);
    @include md-headline-small;
    margin-bottom: 12px;
  }
  &__body {
    flex: 1;
    max-height: 52vh;
    color: var(--md-on-surface-variant);
    @include md-body-medium;
  }
  &__actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 20px;
  }
}
@keyframes m3-fade-in {
  from { opacity: 0; }
  to { opacity: 1; }
}
@keyframes m3-dialog-in {
  from { opacity: 0; transform: scale(0.9); }
  to { opacity: 1; transform: scale(1); }
}
</style>
