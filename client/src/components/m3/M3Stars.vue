<template>
  <view class="m3-stars" :class="{ 'is-input': editable }">
    <view
      v-for="i in 5"
      :key="i"
      class="m3-stars__star"
      @click="editable && $emit('update:modelValue', i)"
    >
      <M3Icon :name="i <= Math.round(modelValue) ? 'star' : 'star_border'" :size="size" :class="i <= Math.round(modelValue) ? 'is-on' : 'is-off'" />
    </view>
    <text v-if="showValue" class="m3-stars__value">{{ valueText }}</text>
  </view>
</template>

<script setup>
// 星级展示/输入（editable 时点击打分）；star=实心，star_rate=空心
import { computed } from 'vue'

const props = defineProps({
  modelValue: { type: Number, default: 0 },
  size: { type: Number, default: 20 },
  editable: Boolean,
  showValue: Boolean
})
const emit = defineEmits(['update:modelValue'])
const valueText = computed(() => Number(props.modelValue || 0).toFixed(1))
</script>

<style lang="scss" scoped>
.m3-stars {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  &__star {
    display: flex;
  }
  &__value {
    margin-left: 6px;
    color: var(--md-on-surface-variant);
    @include md-label-medium;
  }
}
.is-input .m3-stars__star {
  cursor: pointer;
  padding: 2px;
}
.is-on {
  color: var(--md-primary);
}
.is-off {
  color: var(--md-outline-variant);
}
</style>
