<template>
  <view class="md-page">
    <M3TopAppBar title="拼团详情" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 骨架屏 -->
      <template v-if="firstLoading && !gb">
        <M3Card variant="filled" class="gb-skeleton">
          <view class="gb-skeleton-inner">
            <M3Skeleton variant="text" width="55%" height="16" />
            <M3Skeleton variant="text" width="35%" height="14" />
            <M3Skeleton variant="rect" width="100%" height="4" />
            <M3Skeleton variant="rect" width="100%" height="40" />
          </view>
        </M3Card>
      </template>

      <template v-else-if="gb">
        <!-- 团信息卡 -->
        <M3Card variant="filled" class="gb-head">
          <view class="gb-head-inner">
            <view class="gb-title-row">
              <view class="gb-title md-ellipsis-2">
                {{ gb.activityTitle ? '限时拼团 · ' + gb.activityTitle : gb.title }}
              </view>
              <view class="st" :style="{ color: statusColor }">
                <view class="st-dot" :style="{ background: statusColor }" />
                <text class="st-label">{{ statusText }}</text>
              </view>
            </view>
            <view class="gb-price-row">
              <view class="gb-price">
                <text class="gb-price-symbol">¥</text>
                <text class="gb-price-num">{{ yuan(gb.price) }}</text>
              </view>
              <text class="gb-cap">{{ gb.capacity }}人成团</text>
            </view>

            <!-- 进度 -->
            <view class="gb-progress">
              <M3Progress type="linear" :model-value="progress" />
              <view class="gb-progress-meta">
                <text class="gb-count">{{ gb.paidCount }}/{{ gb.capacity }} 人已上车</text>
                <text v-if="gb.status === 1" class="gb-lack">还差 {{ lackCount }} 人</text>
              </view>
            </view>

            <!-- 成员头像：团长 + 已上车成员占位 -->
            <view class="gb-members">
              <view class="gb-member">
                <M3Avatar :name="gb.leaderName" :size="44" />
                <text class="gb-member-role">团长</text>
              </view>
              <view v-for="i in memberSlots" :key="i" class="gb-member">
                <view v-if="i <= gb.paidCount" class="gb-member-filled">
                  <M3Icon name="person" :size="22" />
                </view>
                <view v-else class="gb-member-empty">
                  <text>?</text>
                </view>
                <text class="gb-member-role">成员</text>
              </view>
            </view>
            <view class="gb-end-row">
              <M3Icon name="schedule" :size="14" class="gb-end-icon" />
              <text class="gb-end">{{ gb.endText }} 截止 · 团长：{{ gb.leaderName }}</text>
            </view>

            <!-- 主操作区 -->
            <view class="gb-actions">
              <M3Button
                v-if="gb.canJoin"
                variant="filled"
                icon="group_add"
                block
                :loading="joining"
                @click="onJoin"
              >¥{{ yuan(gb.price) }} 立即参团</M3Button>

              <M3Button
                v-else-if="gb.joined && !gb.myOrderPaid"
                variant="tonal"
                icon="payments"
                block
                :loading="paying"
                @click="onPayAgain"
              >我的订单未支付，去支付</M3Button>

              <!-- 已支付/已成团：邀请好友（MP open-type=share / H5 复制链接） -->
              <template v-else>
                <!-- #ifdef MP-WEIXIN -->
                <button class="mp-share-btn" open-type="share">
                  <M3Icon name="share" :size="18" />
                  <text>{{ gb.myOrderPaid ? '已成团的好友，邀人拼团' : '分享给好友，一起拼' }}</text>
                </button>
                <!-- #endif -->
                <!-- #ifdef H5 -->
                <M3Button variant="filled" icon="share" block @click="onH5Share">
                  {{ gb.myOrderPaid ? '已成团的好友，邀人拼团' : '分享给好友，一起拼' }}
                </M3Button>
                <!-- #endif -->
              </template>
            </view>

            <view v-if="gb.status === 1" class="gb-tip">
              <M3Icon name="info" :size="14" class="gb-tip-icon" />
              <text class="gb-tip-text">到期未成团，已支付款项将自动原路退回</text>
            </view>
          </view>
        </M3Card>
      </template>

      <!-- 加载失败/不存在 -->
      <M3EmptyState v-else icon="groups" title="拼团不存在或已结束" description="回到课包挑一个拼团价吧">
        <M3Button variant="filled" icon="shopping_bag" @click="goCourse">去逛逛</M3Button>
      </M3EmptyState>
    </view>

    <!-- H5 扫码支付弹层 -->
    <M3PaySheet :open="payOpen" :qr="payQr" :amount="payAmount" @close="payOpen = false" />
  </view>
</template>

<script setup>
import { ref, computed } from 'vue'
import { onLoad, onShow, onPageScroll, onShareAppMessage } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { payOrder } from '@/utils/pay'
import { useAuthStore } from '@/stores/auth'
import { yuan, fmtTime } from '@/utils/format'
import { mpSharePayload, shareLink } from '@/utils/referral'

const auth = useAuthStore()
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

const gbid = ref(0)
const gb = ref(null)
const firstLoading = ref(true)
const joining = ref(false)
const paying = ref(false)

const payOpen = ref(false)
const payQr = ref('')
const payAmount = ref('')

onLoad(async (query) => {
  await auth.ensureLogin().catch(() => {})
  gbid.value = parseInt((query && query.gbid) || 0, 10) || 0
})

onShow(() => {
  load()
})

async function load() {
  if (!gbid.value) {
    firstLoading.value = false
    return
  }
  try {
    const data = await request({ url: `/api/v1/group-buys/${gbid.value}` })
    data.endText = fmtTime(data.expireAt).slice(5, 16)
    data.canJoin = data.status === 1 && !data.joined
    gb.value = data
  } catch (e) {
    gb.value = null // request 已 toast
  } finally {
    firstLoading.value = false
  }
}

// 状态：颜色圆点 + label-small（报名中/已成团/未成团）
const statusText = computed(() => {
  const s = gb.value && gb.value.status
  return s === 2 ? '已成团' : s === 3 ? '未成团' : '报名中'
})
const statusColor = computed(() => {
  const s = gb.value && gb.value.status
  if (s === 2) return 'var(--md-primary)'
  if (s === 3) return 'var(--md-error)'
  return 'var(--md-tertiary)'
})

const progress = computed(() => {
  const g = gb.value
  if (!g || !g.capacity) return 0
  return Math.round((g.paidCount * 100) / g.capacity)
})
const lackCount = computed(() => {
  const g = gb.value
  if (!g) return 0
  return Math.max(0, (g.capacity || 0) - (g.paidCount || 0))
})
// 成员占位（不含团长）
const memberSlots = computed(() => {
  const g = gb.value
  return Math.max(0, (g && g.capacity ? g.capacity : 0) - 1)
})

// 分享：参团即裂变落地方（卡片带上团 ID；mpSharePayload 自动附邀请参）
// #ifdef MP-WEIXIN
onShareAppMessage(() => mpSharePayload('/pages/groupbuy/detail?gbid=' + gbid.value, auth.user?.id || 0))
// #endif

// #ifdef H5
function onH5Share() {
  // shareLink 末尾固定拼 '?inviter='，路径已带 query 时把邀请参并进 path（& 连接）
  const uid = auth.user?.id || 0
  shareLink(`/pages/groupbuy/detail?gbid=${gbid.value}${uid ? '&inviter=' + uid : ''}`, 0)
}
// #endif

// 参团：返回待支付订单 → 统一 payOrder 支付
async function onJoin() {
  if (joining.value || !gbid.value) return
  joining.value = true
  try {
    const order = await request({ url: `/api/v1/group-buys/${gbid.value}/join`, method: 'POST', data: {} })
    payAmount.value = yuan(gb.value && gb.value.price)
    const payRes = await payOrder(order.id, {
      onQrCode: (dataUrl) => {
        payQr.value = dataUrl
        payOpen.value = true
      }
    })
    if (payRes.paid) {
      payOpen.value = false
      uni.showToast({ title: '参团成功，等好友上车', icon: 'none' })
    }
    load()
  } catch (e) {
    // 用户取消支付 / 请求层错误（toast 已处理）
  } finally {
    joining.value = false
  }
}

// 未支付订单继续支付
async function onPayAgain() {
  const g = gb.value
  if (!g || !g.myOrderId || paying.value) return
  paying.value = true
  try {
    payAmount.value = yuan(g.price)
    const payRes = await payOrder(g.myOrderId, {
      onQrCode: (dataUrl) => {
        payQr.value = dataUrl
        payOpen.value = true
      }
    })
    if (payRes.paid) {
      payOpen.value = false
      uni.showToast({ title: '支付成功', icon: 'success' })
    }
    load()
  } catch (e) {
    // 用户取消支付 / 请求层错误（toast 已处理）
  } finally {
    paying.value = false
  }
}

function goCourse() {
  uni.reLaunch({ url: '/pages/course/list' })
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

.gb-skeleton {
  margin-bottom: 12px;
}
.gb-skeleton-inner {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.gb-head {
  margin-bottom: 16px;
}
.gb-head-inner {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.gb-title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}
.gb-title {
  flex: 1;
  min-width: 0;
  color: var(--md-on-surface);
  @include md-title-large;
  line-height: 26px;
}

// 状态：颜色圆点 + label-small
.st {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  &-dot {
    width: 8px;
    height: 8px;
    border-radius: $md-shape-full;
  }
  &-label {
    @include md-label-small;
  }
}

.gb-price-row {
  display: flex;
  align-items: baseline;
  gap: 10px;
}
.gb-price {
  display: flex;
  align-items: baseline;
  gap: 2px;
  &-symbol {
    @include md-title-medium;
    font-weight: 700;
    color: var(--md-primary);
  }
  &-num {
    @include md-headline-medium;
    font-weight: 700;
    color: var(--md-primary);
  }
}
.gb-cap {
  color: var(--md-on-surface-variant);
  @include md-body-medium;
}

.gb-progress {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.gb-progress-meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.gb-count {
  color: var(--md-on-surface-variant);
  @include md-label-medium;
}
.gb-lack {
  color: var(--md-tertiary);
  @include md-label-medium;
  font-weight: 500;
}

// 成员头像
.gb-members {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.gb-member {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  &-role {
    @include md-label-small;
    color: var(--md-on-surface-variant);
  }
}
.gb-member-filled {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);
  display: flex;
  align-items: center;
  justify-content: center;
}
.gb-member-empty {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  border: 1px dashed var(--md-outline-variant);
  color: var(--md-outline);
  display: flex;
  align-items: center;
  justify-content: center;
  @include md-title-medium;
}

.gb-end-row {
  display: flex;
  align-items: center;
  gap: 4px;
}
.gb-end-icon {
  color: var(--md-on-surface-variant);
}
.gb-end {
  color: var(--md-on-surface-variant);
  @include md-body-small;
}

.gb-actions {
  margin-top: 4px;
}

// MP 原生分享按钮（open-type=share，重置原生样式为 MD3 filled 观感）
.mp-share-btn {
  width: 100%;
  height: 40px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  margin: 0;
  border: none;
  border-radius: $md-shape-full;
  background: var(--md-primary);
  color: var(--md-on-primary);
  @include md-label-large;
  font-weight: 600;
  &::after {
    border: none;
  }
}

.gb-tip {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  border-radius: $md-shape-sm;
  background: var(--md-surface-container-highest);
}
.gb-tip-icon {
  color: var(--md-on-surface-variant);
  flex-shrink: 0;
}
.gb-tip-text {
  color: var(--md-on-surface-variant);
  @include md-body-small;
}
</style>
