<template>
  <M3BottomSheet :open="open" @close="$emit('close')">
    <view class="m3-pay">
      <text class="m3-pay__title">微信扫码支付</text>
      <text class="m3-pay__amount" v-if="amount">¥{{ amount }}</text>
      <view class="m3-pay__qr">
        <image v-if="qr" :src="qr" class="m3-pay__qr-img" mode="aspectFit" />
        <view v-else class="m3-pay__qr-loading">
          <M3Progress type="circular" :size="36" indeterminate />
        </view>
      </view>
      <text class="m3-pay__hint">{{ hint || '请使用微信扫一扫完成支付' }}</text>
      <text class="m3-pay__sub">支付完成后页面将自动更新</text>
      <view class="m3-pay__actions">
        <M3Button variant="text" @click="$emit('close')">暂不支付</M3Button>
      </view>
    </view>
  </M3BottomSheet>
</template>

<script setup>
// H5 Native 扫码支付弹层（MP 端不使用，MP 走 uni.requestPayment）
defineProps({
  open: Boolean,
  qr: { type: String, default: '' }, // 二维码 dataURL
  amount: { type: String, default: '' }, // 展示金额（元）
  hint: { type: String, default: '' }
})
defineEmits(['close'])
</script>

<style lang="scss" scoped>
.m3-pay {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 8px 0 8px;
  &__title {
    @include md-title-medium;
    color: var(--md-on-surface);
  }
  &__amount {
    margin-top: 4px;
    color: var(--md-primary);
    @include md-headline-medium;
  }
  &__qr {
    margin: 20px 0;
    width: 220px;
    height: 220px;
    border-radius: $md-shape-md;
    background: var(--md-surface-container-lowest);
    border: 1px solid var(--md-outline-variant);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  &__qr-img {
    width: 200px;
    height: 200px;
  }
  &__hint {
    @include md-body-medium;
    color: var(--md-on-surface);
  }
  &__sub {
    margin-top: 4px;
    @include md-body-small;
    color: var(--md-on-surface-variant);
  }
  &__actions {
    margin-top: 12px;
  }
}
</style>
