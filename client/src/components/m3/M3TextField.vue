<template>
  <view class="m3-tf" :class="{ 'is-error': !!error }">
    <view class="m3-tf__field" :class="{ 'is-focused': focused, 'is-disabled': disabled }">
      <text
        class="m3-tf__label"
        :class="{ 'is-float': focused || !!modelValue || placeholder }"
        :style="{ background: labelBg }"
      >{{ label }}</text>
      <textarea
        v-if="multiline"
        class="m3-tf__input m3-tf__input--area"
        :value="modelValue"
        :placeholder="placeholder"
        :placeholder-class="'m3-tf-ph'"
        :disabled="disabled"
        :maxlength="maxlength"
        :auto-height="autoHeight"
        @input="onInput"
        @focus="focused = true"
        @blur="focused = false"
      />
      <input
        v-else
        class="m3-tf__input"
        :value="modelValue"
        :type="type === 'password' ? 'text' : type"
        :password="type === 'password'"
        :placeholder="placeholder"
        :placeholder-class="'m3-tf-ph'"
        :disabled="disabled"
        :maxlength="maxlength"
        @input="onInput"
        @focus="focused = true"
        @blur="focused = false"
        @confirm="$emit('confirm')"
      />
      <view v-if="type === 'password'" class="m3-tf__trail" @click="showPwd = !showPwd">
        <M3Icon :name="showPwd ? 'visibility_off' : 'visibility'" :size="22" />
      </view>
      <slot name="trail" />
    </view>
    <text v-if="error" class="m3-tf__msg is-error-text">{{ error }}</text>
    <text v-else-if="hint" class="m3-tf__msg">{{ hint }}</text>
  </view>
</template>

<script setup>
// MD3 外描边输入框：聚焦收拢描边 + 浮动标签
import { ref } from 'vue'

defineProps({
  modelValue: { type: [String, Number], default: '' },
  label: { type: String, default: '' },
  placeholder: { type: String, default: '' },
  type: { type: String, default: 'text' }, // text | number | password
  multiline: Boolean,
  autoHeight: Boolean,
  maxlength: { type: Number, default: 140 },
  disabled: Boolean,
  error: { type: String, default: '' },
  hint: { type: String, default: '' },
  // 标签覆盖描边时的底色，卡片内使用时传卡片背景色
  labelBg: { type: String, default: 'var(--md-surface)' }
})
const emit = defineEmits(['update:modelValue', 'confirm'])
const focused = ref(false)
const showPwd = ref(false)

function onInput(e) {
  emit('update:modelValue', e.detail ? e.detail.value : e.target.value)
}
</script>

<style lang="scss" scoped>
.m3-tf {
  &__field {
    position: relative;
    display: flex;
    align-items: center;
    border: 1px solid var(--md-outline);
    border-radius: $md-shape-xs;
    min-height: 52px;
    padding: 0 14px;
    transition: border-color $md-motion-short;
    &.is-focused {
      border: 2px solid var(--md-primary);
      padding: 0 13px; // 2px 边框补差
    }
    &.is-disabled {
      opacity: 0.4;
    }
    &.is-error,
    &.is-error.is-focused {
      border-color: var(--md-error);
    }
  }
  &__label {
    position: absolute;
    left: 12px;
    top: 50%;
    transform: translateY(-50%);
    padding: 0 4px;
    color: var(--md-on-surface-variant);
    @include md-body-large;
    transition: all $md-motion-short $md-motion-standard;
    pointer-events: none;
    &.is-float {
      top: 0;
      transform: translateY(-50%) scale(0.75);
      transform-origin: left center;
      color: var(--md-primary);
      @include md-body-large;
    }
    .is-error &,
    .is-error &.is-float {
      color: var(--md-error);
    }
  }
  &__input {
    flex: 1;
    min-width: 0;
    height: 50px;
    color: var(--md-on-surface);
    @include md-body-large;
    &--area {
      padding: 12px 0;
      height: auto;
      min-height: 80px;
      line-height: 20px;
    }
  }
  &__trail {
    display: flex;
    align-items: center;
    color: var(--md-on-surface-variant);
    cursor: pointer;
  }
  &__msg {
    margin: 4px 16px 0;
    @include md-body-small;
    color: var(--md-on-surface-variant);
    &.is-error-text {
      color: var(--md-error);
    }
  }
}
.m3-tf-ph {
  color: var(--md-on-surface-variant);
}
</style>
