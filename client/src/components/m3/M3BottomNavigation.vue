<template>
  <view class="m3-bnav-space">
    <view class="m3-bnav">
      <view
        v-for="it in items"
        :key="it.key"
        class="m3-bnav__item"
        :class="{ 'is-active': it.key === modelValue }"
        @click="onTap(it)"
      >
        <view class="m3-bnav__icon-wrap">
          <M3Icon :name="it.icon" :size="24" />
        </view>
        <text class="m3-bnav__label">{{ it.label }}</text>
      </view>
    </view>
  </view>
</template>

<script setup>
// MD3 底部导航（自绘；tab 切换由页面监听 change 后 uni.reLaunch，见 DESIGN.md）
const props = defineProps({
  items: { type: Array, default: () => [] }, // [{key, icon, label}]
  modelValue: { type: String, default: '' }
})
const emit = defineEmits(['update:modelValue', 'change'])

function onTap(it) {
  if (it.key === props.modelValue) return
  emit('update:modelValue', it.key)
  emit('change', it)
}
</script>

<style lang="scss" scoped>
.m3-bnav-space {
  // 占位 = 栏高 64px + 底部安全区 + 内边距 8px
  height: calc(72px + env(safe-area-inset-bottom));
}
.m3-bnav {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 100;
  background: var(--md-surface-container);
  padding-bottom: calc(env(safe-area-inset-bottom) + 8px);
  @include md-elevation-2;
  display: flex;
  justify-content: center;
  &__item {
    flex: 1;
    max-width: 160px;
    display: flex;
    flex-direction: column;
    align-items: center;
    padding-top: 8px;
    cursor: pointer;
    color: var(--md-on-surface-variant);
  }
  &__icon-wrap {
    width: 56px;
    height: 30px;
    border-radius: $md-shape-full;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background $md-motion-medium $md-motion-emphasized;
  }
  &__item.is-active {
    color: var(--md-on-secondary-container);
    .m3-bnav__icon-wrap {
      background: var(--md-secondary-container);
    }
  }
  &__label {
    margin-top: 4px;
    @include md-label-medium;
  }
}
</style>
