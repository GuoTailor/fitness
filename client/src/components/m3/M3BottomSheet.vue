<template>
  <view v-if="open" class="m3-sheet-scrim" @click="onScrim">
    <view class="m3-sheet" @click.stop>
      <view class="m3-sheet__handle" />
      <view v-if="title" class="m3-sheet__title">{{ title }}</view>
      <scroll-view scroll-y class="m3-sheet__body">
        <slot />
      </scroll-view>
    </view>
  </view>
</template>

<script setup>
// MD3 底部弹层（原小程序 ActionSheet 的替代）
const props = defineProps({
  open: Boolean,
  title: { type: String, default: '' }
})
const emit = defineEmits(['close'])

function onScrim() {
  emit('close')
}
</script>

<style lang="scss" scoped>
.m3-sheet-scrim {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: var(--md-scrim);
  display: flex;
  align-items: flex-end;
  justify-content: center;
  animation: m3-fade $md-motion-short both;
}
.m3-sheet {
  width: 100%;
  max-width: #{$md-page-max-width};
  max-height: 72vh;
  background: var(--md-surface-container-low);
  border-radius: $md-shape-xl $md-shape-xl 0 0;
  padding: 8px 16px calc(16px + env(safe-area-inset-bottom));
  display: flex;
  flex-direction: column;
  animation: m3-slide-up $md-motion-medium $md-motion-emphasized both;
  &__handle {
    width: 32px;
    height: 4px;
    border-radius: $md-shape-full;
    background: var(--md-outline-variant);
    margin: 8px auto 4px;
  }
  &__title {
    @include md-title-medium;
    color: var(--md-on-surface);
    padding: 8px 0;
  }
  &__body {
    flex: 1;
    max-height: 62vh;
  }
}
@keyframes m3-fade {
  from { opacity: 0; }
  to { opacity: 1; }
}
@keyframes m3-slide-up {
  from { transform: translateY(40%); opacity: 0.6; }
  to { transform: translateY(0); opacity: 1; }
}
</style>
