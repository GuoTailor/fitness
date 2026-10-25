<template>
  <view class="m3-appbar-space">
    <view class="m3-appbar" :class="{ 'is-elevated': elevated }">
      <view class="m3-appbar__row">
        <view class="m3-appbar__lead">
          <M3IconButton v-if="back" icon="arrow_back" @click="goBack" />
          <slot name="leading" />
        </view>
        <text class="m3-appbar__title md-ellipsis">{{ title }}</text>
        <view class="m3-appbar__actions">
          <slot name="actions" />
        </view>
      </view>
    </view>
  </view>
</template>

<script setup>
// MD3 顶部应用栏：固定栏 + 文档流占位一体渲染，页面无需自行留白
// 状态栏高度：小程序端用 --status-bar-height；H5 端为 0
defineProps({
  title: { type: String, default: '' },
  back: Boolean,
  elevated: Boolean // 页面 onPageScroll 里滚动超过 8px 时传 true
})

function goBack() {
  const pages = getCurrentPages()
  if (pages.length > 1) {
    uni.navigateBack()
  } else {
    uni.reLaunch({ url: '/pages/index/index' })
  }
}
</script>

<style lang="scss" scoped>
.m3-appbar-space {
  // 占位 = 状态栏 + 栏高（64px）
  height: calc(var(--status-bar-height, 0px) + 64px);
}
.m3-appbar {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 100;
  padding-top: var(--status-bar-height, 0px);
  background: var(--md-surface);
  transition: box-shadow $md-motion-medium $md-motion-standard, background $md-motion-medium;
  &.is-elevated {
    background: var(--md-surface-container);
    @include md-elevation-2;
  }
  &__row {
    max-width: #{$md-page-max-width};
    margin: 0 auto;
    height: 64px;
    display: flex;
    align-items: center;
    padding: 0 8px;
  }
  &__lead {
    display: flex;
    align-items: center;
    min-width: 40px;
  }
  &__title {
    flex: 1;
    min-width: 0;
    margin-left: 4px;
    color: var(--md-on-surface);
    @include md-title-large;
  }
  &__actions {
    display: flex;
    align-items: center;
    gap: 4px;
  }
}
</style>
