<template>
  <view class="md-page">
    <M3TopAppBar title="课包详情" :back="true" :elevated="scrolled">
      <template #actions>
        <!-- #ifdef H5 -->
        <M3IconButton icon="share" @click="onShareH5" />
        <!-- #endif -->
      </template>
    </M3TopAppBar>

    <view class="md-shell page-body">
      <!-- 首屏骨架 -->
      <template v-if="!detail && !loadFailed">
        <M3Skeleton variant="rect" height="210px" />
        <M3Card variant="filled" class="card">
          <M3Skeleton variant="text" height="22" />
          <view class="skeleton-gap" />
          <M3Skeleton variant="text" width="180" height="14" />
        </M3Card>
        <M3Card variant="filled" class="card">
          <M3Skeleton variant="text" width="120" height="16" />
          <view class="skeleton-gap" />
          <M3Skeleton variant="rect" height="72px" />
        </M3Card>
      </template>

      <!-- 加载失败 -->
      <M3EmptyState
        v-else-if="loadFailed"
        icon="error"
        title="加载失败"
        description="网络似乎不太顺畅，稍后再试试"
      >
        <M3Button variant="filled" @click="load">重新加载</M3Button>
      </M3EmptyState>

      <template v-else-if="detail">
        <!-- 视频封面 -->
        <video
          v-if="pkg.introVideoUrl"
          class="media"
          :src="pkg.introVideoUrl"
          object-fit="cover"
        />
        <image v-else class="media" :src="pkg.coverUrl" mode="aspectFill" />

        <!-- 标题卡 -->
        <M3Card variant="filled" class="card">
          <view class="pkg-title">{{ pkg.title }}</view>
          <view class="meta-row">
            <text v-for="t in pkgTags" :key="t" class="meta-tag">{{ t }}</text>
            <text class="meta-diff">难度 {{ pkg.difficulty }}/5</text>
            <view v-if="detail.ratingCount" class="meta-rating">
              <M3Icon name="star" :size="14" class="rating-star" />
              <text class="meta-rating-text">{{ detail.ratingAvg }}（{{ detail.ratingCount }}条评价）</text>
            </view>
          </view>
        </M3Card>

        <!-- 规格卡 -->
        <M3Card variant="filled" class="card">
          <view class="section-title">选择规格</view>
          <view class="sku-list">
            <view
              v-for="(s, i) in detail.skus"
              :key="s.id"
              class="sku"
              :class="{ 'is-active': selectedSku === i }"
              @click="onSkuSelect(i)"
            >
              <view class="sku-name">{{ s.name }}</view>
              <view class="sku-sub">{{ s.sessions }}节 · 有效期{{ s.validDays }}天</view>
              <view class="sku-price">
                <template v-if="s.dispYuan">
                  <text class="np">¥{{ s.dispYuan }}</text>
                  <text class="strike">¥{{ s.priceYuan }}</text>
                  <text class="np-tag" :class="{ 'is-promo': s.dispTag === '限时价' }">{{ s.dispTag }}</text>
                </template>
                <text v-else class="sku-price-plain">¥{{ s.priceYuan }}</text>
              </view>
            </view>
          </view>

          <!-- 优惠券 -->
          <view v-if="sku" class="coupon-bar" @click="openCouponSheet">
            <text class="c-label">优惠券</text>
            <text class="c-value md-ellipsis">{{ couponText }}</text>
            <text class="c-go" @click.stop="goCouponCenter">领券</text>
            <M3Icon name="chevron_right" :size="20" class="c-arrow" />
          </view>

          <!-- 限时活动倒计时 -->
          <view v-if="sku && sku.promoActive && promoLeft" class="promo-count">
            限时活动 · 距结束 {{ promoLeft }}
          </view>

          <view v-if="sku && (sku.newcomerYuan || sku.promoActive)" class="np-tip">
            新人价/限时活动价与优惠券不叠加，自动按最优结算；赠课券可叠加（购课赠课时）
          </view>

          <!-- 拼团入口：SKU 配置了拼团价时展示 -->
          <view v-if="sku && sku.groupPrice > 0" class="gb-bar">
            <view class="gb-info">
              <view class="gb-line">
                <text class="gb-price">{{ sku.groupCapacity }}人团 ¥{{ yuan(sku.groupPrice) }}</text>
                <text v-if="sku.groupActivityTitle" class="gb-act">限时 · {{ sku.groupActivityTitle }}</text>
              </view>
              <text class="gb-tip">拼团价不与券/新人价叠加，未成团自动退款</text>
            </view>
            <M3Button variant="tonal" size="sm" @click="goGroupBuy">去拼团</M3Button>
          </view>
        </M3Card>

        <!-- 大纲卡 -->
        <M3Card variant="filled" class="card">
          <view class="section-title">课程大纲</view>
          <view class="rich">{{ pkg.outline || '暂无介绍' }}</view>
          <view class="section-title section-title-gap">适合人群</view>
          <view class="rich">{{ pkg.suitable || '—' }}</view>
        </M3Card>

        <!-- 教练卡 -->
        <M3Card variant="filled" class="card">
          <view class="section-title">授课教练</view>
          <view
            v-for="c in detail.coaches"
            :key="c.id"
            class="coach-row"
            @click="goCoach(c.id)"
          >
            <M3Avatar :src="c.avatarUrl" :name="c.name" :size="44" />
            <view class="coach-info">
              <view class="coach-name">
                <text>{{ c.name }}</text>
                <view v-if="c.certVerified" class="cert">
                  <M3Icon name="verified" :size="12" />
                  <text>已认证</text>
                </view>
              </view>
              <view class="coach-sub md-ellipsis">{{ c.specialties }} · {{ c.years }}年教龄</view>
            </view>
            <M3Icon name="chevron_right" :size="20" class="coach-arrow" />
          </view>
          <view v-if="!detail.coaches.length" class="empty-inline">暂未关联教练</view>
        </M3Card>

        <!-- 评价 / 问答卡 -->
        <M3Card variant="filled" class="card">
          <M3Tabs
            :tabs="tabItems"
            :model-value="reviewTab"
            @update:model-value="onTabChange"
          />

          <!-- 学员评价 -->
          <view v-if="reviewTab === 'reviews'" class="tab-body">
            <view v-for="r in detail.reviews" :key="r.id" class="review">
              <view class="review-head">
                <M3Stars :model-value="r.rating" :size="14" />
                <text class="review-time">{{ r.createdAt }}</text>
              </view>
              <view class="review-content">{{ r.content }}</view>
              <view v-if="r.coachReply" class="review-reply">教练回复：{{ r.coachReply }}</view>
            </view>
            <view v-if="!detail.reviews.length" class="empty-inline">暂无评价，完成上课后即可评价</view>
            <view
              v-if="detail.ratingCount > 5"
              class="all-reviews"
              @click="goPublicReviews"
            >
              查看全部 {{ detail.ratingCount }} 条评价
              <M3Icon name="chevron_right" :size="16" />
            </view>
          </view>

          <!-- 课程问答 -->
          <view v-else class="tab-body">
            <view class="qa-ask-bar" @click="askOpen = !askOpen">
              <text class="qa-ask-tip">想了解这门课？提问前先看看已有问答</text>
              <M3Icon :name="askOpen ? 'expand_less' : 'expand_more'" :size="20" class="qa-ask-icon" />
            </view>

            <view v-if="askOpen" class="qa-form">
              <M3TextField
                v-model="askContent"
                label="提问"
                placeholder="例如：零基础可以上吗？需要准备什么？"
                multiline
                :maxlength="500"
                label-bg="var(--md-surface-container-high)"
              />
              <view class="qa-form-foot">
                <view class="qa-anon">
                  <text class="qa-anon-label">匿名展示</text>
                  <M3Switch v-model="askAnonymous" />
                </view>
                <M3Button variant="filled" size="sm" :loading="asking" @click="submitAsk">
                  提交提问
                </M3Button>
              </view>
            </view>

            <view v-for="q in qaList" :key="q.id" class="qa-item">
              <view class="qa-row">
                <view class="qa-badge is-q">问</view>
                <view class="qa-body">
                  <view class="qa-content">{{ q.content }}</view>
                  <view class="qa-meta">{{ q.asker }} · {{ q.timeText }}</view>
                </view>
                <text v-if="q.pinned" class="qa-pin">置顶</text>
              </view>
              <view
                v-for="a in q.answers"
                :key="a.id"
                class="qa-row qa-answer"
              >
                <view class="qa-badge" :class="a.isCoach ? 'is-coach' : 'is-a'">
                  {{ a.isCoach ? '教练' : '答' }}
                </view>
                <view class="qa-body">
                  <view class="qa-content is-answer">{{ a.content }}</view>
                  <view class="qa-meta">{{ a.authorName || '学员' }} · {{ a.timeText }}</view>
                </view>
              </view>
              <view v-if="!q.answers || !q.answers.length" class="qa-none">
                等待教练或已购学员回复
              </view>
            </view>
            <view v-if="!qaList.length && !asking" class="empty-inline">
              暂无提问，来问第一个问题吧
            </view>
          </view>
        </M3Card>

        <!-- 底部操作栏占位 -->
        <view class="footer-space" />
      </template>
    </view>

    <!-- 底部固定操作栏 -->
    <view v-if="detail" class="footer-bar">
      <view class="md-shell footer-inner">
        <view v-if="payYuan" class="pay-amount">
          <text class="pa-label">预估合计</text>
          <text class="pa-num">¥{{ payYuan }}</text>
        </view>
        <M3Button
          variant="filled"
          size="lg"
          class="buy-btn"
          :loading="buying"
          :disabled="selectedSku < 0"
          @click="onBuy"
        >
          {{ selectedSku < 0 ? '请选择规格' : '立即购买' }}
        </M3Button>
      </view>
    </view>

    <!-- 优惠券选择弹层（原 ActionSheet） -->
    <M3BottomSheet :open="couponOpen" title="选择优惠券" @close="couponOpen = false">
      <M3ListItem
        headline="不使用优惠券"
        clickable
        :trailing-icon="couponIndex === -1 ? 'check' : ''"
        @click="pickCoupon(-1)"
      />
      <M3ListItem
        v-for="(c, i) in coupons"
        :key="c.couponId"
        :headline="c.name + ' ' + c.label"
        :supporting="c.giftSessions ? '购课赠课时，价格不下调' : '立减 ¥' + c.benefitYuan"
        clickable
        :trailing-icon="couponIndex === i ? 'check' : ''"
        @click="pickCoupon(i)"
      />
    </M3BottomSheet>

    <!-- H5 扫码支付弹层 -->
    <M3PaySheet
      :open="payOpen"
      :qr="payQr"
      :amount="payYuan"
      hint="请使用微信扫一扫完成支付"
      @close="payOpen = false"
    />

    <!-- 支付成功 -->
    <M3Dialog
      :open="successOpen"
      title="支付成功"
      icon="check_circle"
      @close="successOpen = false"
    >
      <text class="success-text">{{ successText }}</text>
      <template #actions>
        <M3Button variant="text" @click="successOpen = false">看课包</M3Button>
        <M3Button variant="filled" @click="goTraining">去预约</M3Button>
      </template>
    </M3Dialog>
  </view>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { onLoad, onShow, onHide, onUnload, onPageScroll, onShareAppMessage } from '@dcloudio/uni-app'
import { request } from '@/utils/request'
import { payOrder } from '@/utils/pay'
import { useAuthStore } from '@/stores/auth'
import { yuan, yuanFixed, fmtTime } from '@/utils/format'
// #ifdef MP-WEIXIN
import { mpSharePayload } from '@/utils/referral'
// #endif
// #ifdef H5
import { shareLink } from '@/utils/referral'
// #endif

const auth = useAuthStore()

const id = ref(0)
const detail = ref(null)
const loadFailed = ref(false)
const selectedSku = ref(-1)
const coupons = ref([]) // 当前 SKU 可用券（后端已按门槛/范围过滤）
const couponIndex = ref(-1) // -1 = 不使用优惠券
const couponOpen = ref(false)
const buying = ref(false)
const reviewTab = ref('reviews')
// 问答
const qaList = ref([])
const qaTotal = ref(0)
const askOpen = ref(false)
const askContent = ref('')
const askAnonymous = ref(false)
const asking = ref(false)
// 支付
const payOpen = ref(false)
const payQr = ref('')
const successOpen = ref(false)
const successText = ref('')
// 限时活动倒计时
const promoLeft = ref('')

const scrolled = ref(false)
onPageScroll((e) => {
  scrolled.value = e.scrollTop > 8
})

onLoad(async (query) => {
  id.value = query.id
  // 登录态：MP 静默登录；H5 未登录跳登录页（throw 已 catch）
  await auth.ensureLogin().catch(() => {})
  load()
})

// #ifdef MP-WEIXIN
// 老带新：课包详情转发卡带上邀请码
onShareAppMessage(() => mpSharePayload('/pages/course/detail?id=' + id.value, auth.user?.id || 0))
// #endif

// #ifdef H5
function onShareH5() {
  shareLink('/pages/course/detail?id=' + id.value, auth.user?.id || 0)
}
// #endif

const pkg = computed(() => (detail.value && detail.value.package) || {})
const pkgTags = computed(() => (pkg.value.tags ? String(pkg.value.tags).split(',') : []))
const sku = computed(() =>
  detail.value && selectedSku.value >= 0 ? detail.value.skus[selectedSku.value] : null
)

const tabItems = computed(() => [
  { value: 'reviews', label: `学员评价（${(detail.value && detail.value.ratingCount) || 0}）` },
  { value: 'qa', label: qaTotal.value ? `课程问答（${qaTotal.value}）` : '课程问答' }
])

// 预估合计（新人价/活动价与券取低，最终以下单服务端为准）
const payYuan = computed(() => {
  const s = sku.value
  if (!s) return ''
  let pay = s.price
  const c = couponIndex.value >= 0 ? coupons.value[couponIndex.value] : null
  if (c && !c.giftSessions) pay = c.finalPay // 赠课券不降价
  if (s.newcomerEligible && s.newcomerPrice && s.newcomerPrice < pay) pay = s.newcomerPrice
  if (s.promoActive && s.promoPrice && s.promoPrice < pay) pay = s.promoPrice
  return yuanFixed(pay)
})

const couponText = computed(() => {
  const c = couponIndex.value >= 0 ? coupons.value[couponIndex.value] : null
  if (c) {
    return c.giftSessions ? `${c.name} ${c.label}` : `${c.name} ${c.label}（省¥${c.benefitYuan}）`
  }
  return coupons.value.length ? '不使用优惠券' : '暂无可用券'
})

async function load() {
  loadFailed.value = false
  try {
    const data = await request({ url: `/api/v1/packages/${id.value}` })
    // 价格分转元展示；新人价仅对可享用户展示、活动价仅活动期内展示（均服务端判定）
    const skus = (data.skus || []).map((s) => {
      const out = { ...s, priceYuan: (s.price / 100).toFixed(0) }
      const promo = s.promoActive && s.promoPrice ? s.promoPrice : 0
      const nc = s.newcomerEligible && s.newcomerPrice ? s.newcomerPrice : 0
      if (nc && (!promo || nc < promo)) {
        out.newcomerYuan = (nc / 100).toFixed(0)
      }
      // 主展示价：新人价更低用新人价，否则活动价（同额展示活动价，与结算规则一致）
      if (out.newcomerYuan) {
        out.dispYuan = out.newcomerYuan
        out.dispTag = '新人价'
      } else if (promo) {
        out.dispYuan = (promo / 100).toFixed(0)
        out.dispTag = '限时价'
      }
      return out
    })
    detail.value = { ...data, skus }
    selectedSku.value = skus.length === 1 ? 0 : -1
    if (skus.length === 1) loadCoupons(skus[0].id)
    startCountdown()
  } catch (e) {
    loadFailed.value = true
  }
}

function onSkuSelect(i) {
  selectedSku.value = i
  const s = detail.value.skus[i]
  loadCoupons(s.id)
  startCountdown()
}

function onTabChange(v) {
  reviewTab.value = v
}

watch(reviewTab, (v) => {
  if (v === 'qa' && !qaList.value.length) loadQa()
})

// ---- 课程问答 ----
async function loadQa() {
  try {
    const data = await request({ url: `/api/v1/packages/${id.value}/qa?page=1&pageSize=20` })
    qaList.value = (data.list || []).map((q) => ({
      ...q,
      timeText: fmtTime(q.createdAt),
      answers: (q.answers || []).map((a) => ({ ...a, timeText: fmtTime(a.createdAt) }))
    }))
    qaTotal.value = data.total || 0
  } catch (e) {
    // 静默：问答加载失败不阻塞详情
  }
}

async function submitAsk() {
  try {
    await auth.ensureLogin()
  } catch (e) {
    return // H5 已跳登录页
  }
  if (askContent.value.trim().length < 5) {
    uni.showToast({ title: '问题至少 5 个字', icon: 'none' })
    return
  }
  asking.value = true
  try {
    await request({
      url: '/api/v1/qa/questions',
      method: 'POST',
      data: {
        packageId: Number(id.value),
        content: askContent.value.trim(),
        anonymous: askAnonymous.value
      }
    })
    uni.showToast({ title: '已提交', icon: 'success' })
    askOpen.value = false
    askContent.value = ''
    askAnonymous.value = false
    loadQa()
  } catch (e) {
    // toast 已处理
  } finally {
    asking.value = false
  }
}

// ---- 优惠券 ----
async function loadCoupons(skuId) {
  try {
    const data = await request({ url: `/api/v1/coupons/usable?skuId=${skuId}` })
    coupons.value = (data.items || []).map((c) => ({ ...c, benefitYuan: (c.benefit / 100).toFixed(2) }))
    couponIndex.value = coupons.value.length ? 0 : -1
  } catch (e) {
    coupons.value = []
    couponIndex.value = -1
  }
}

function openCouponSheet() {
  if (!coupons.value.length) {
    uni.showToast({ title: '该课包暂无可用券，可去领券中心看看', icon: 'none' })
    return
  }
  couponOpen.value = true
}

function pickCoupon(i) {
  couponIndex.value = i
  couponOpen.value = false
}

function goCouponCenter() {
  uni.navigateTo({ url: '/pages/coupon/center' })
}

// ---- 拼团入口：跳转该 SKU 的开团/参团列表 ----
function goGroupBuy() {
  uni.navigateTo({ url: `/pages/groupbuy/list?pkgId=${id.value}&skuId=${sku.value.id}` })
}

function goCoach(coachId) {
  uni.navigateTo({ url: '/pages/coach/detail?id=' + coachId })
}

function goPublicReviews() {
  uni.navigateTo({ url: `/pages/review/public?type=package&id=${pkg.value.id}` })
}

function goTraining() {
  successOpen.value = false
  uni.navigateTo({ url: '/pages/training/index' })
}

// ---- 限时活动倒计时 ----
let promoTimer = null
function startCountdown() {
  stopCountdown()
  const tick = () => {
    const s = sku.value
    if (!s || !s.promoActive || !s.promoEnd) {
      promoLeft.value = ''
      return
    }
    const left = new Date(s.promoEnd).getTime() - Date.now()
    if (left <= 0) {
      promoLeft.value = ''
      return
    }
    const d = Math.floor(left / 86400000)
    const h = String(Math.floor((left % 86400000) / 3600000)).padStart(2, '0')
    const m = String(Math.floor((left % 3600000) / 60000)).padStart(2, '0')
    const sec = String(Math.floor((left % 60000) / 1000)).padStart(2, '0')
    promoLeft.value = (d > 0 ? d + '天' : '') + h + ':' + m + ':' + sec
  }
  tick()
  promoTimer = setInterval(tick, 1000)
}
function stopCountdown() {
  if (promoTimer) {
    clearInterval(promoTimer)
    promoTimer = null
  }
}

onShow(() => startCountdown())
onHide(() => stopCountdown())
onUnload(() => stopCountdown())

// ---- 下单支付（统一 payOrder：MP 拉起收银台 / H5 扫码） ----
async function onBuy() {
  try {
    await auth.ensureLogin()
  } catch (e) {
    return // H5 已跳登录页
  }
  if (selectedSku.value < 0) {
    uni.showToast({ title: '请选择规格', icon: 'none' })
    return
  }
  const s = sku.value
  const c = couponIndex.value >= 0 ? coupons.value[couponIndex.value] : null
  const couponId = c ? c.couponId : 0
  buying.value = true
  try {
    const order = await request({
      url: '/api/v1/orders',
      method: 'POST',
      data: { skuId: s.id, couponId }
    })
    const res = await payOrder(order.id, {
      onQrCode: (dataUrl) => {
        payQr.value = dataUrl
        payOpen.value = true
      }
    })
    payOpen.value = false
    if (res.paid) {
      successText.value = `已购买「${order.title}」，实付 ¥${yuanFixed(order.amount)}`
      successOpen.value = true
    } else {
      uni.showToast({
        title: res.timeout ? '支付超时，可在「我的订单」继续支付' : '支付未完成',
        icon: 'none'
      })
    }
  } catch (e) {
    payOpen.value = false
    // 请求层错误已 toast；这里仅提示支付封装自身的错误（如二维码生成失败）
    if (e && e.message && e.message.indexOf('二维码') >= 0) {
      uni.showToast({ title: e.message, icon: 'none' })
    }
  } finally {
    buying.value = false
  }
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

.media {
  width: 100%;
  height: 210px;
  border-radius: $md-shape-lg;
  background: var(--md-surface-container-highest);
  display: block;
}

.card {
  margin-top: 12px;
  padding: 16px;
}

.pkg-title {
  @include md-headline-small;
  color: var(--md-on-surface);
}

.meta-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin-top: 10px;
}

.meta-tag {
  @include md-label-small;
  background: var(--md-primary-container);
  color: var(--md-on-primary-container);
  border-radius: $md-shape-xs;
  padding: 2px 8px;
}

.meta-diff {
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

.meta-rating {
  display: flex;
  align-items: center;
  gap: 2px;
}

.rating-star {
  color: var(--md-tertiary);
}

.meta-rating-text {
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

.section-title {
  @include md-title-medium;
  color: var(--md-on-surface);
  margin-bottom: 12px;
}

.section-title-gap {
  margin-top: 20px;
}

// ---- SKU ----
.sku-list {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.sku {
  min-width: 108px;
  border: 1px solid var(--md-outline-variant);
  border-radius: $md-shape-md;
  padding: 10px 14px;
  cursor: pointer;
  background: var(--md-surface);
  transition: border-color $md-motion-short, background $md-motion-short;

  &:active {
    background: var(--md-surface-container-highest);
  }

  &.is-active {
    border: 2px solid var(--md-primary);
    background: var(--md-primary-container);
    padding: 9px 13px; // 2px 边框补差
  }
}

.sku-name {
  @include md-title-small;
  color: var(--md-on-surface);
}

.sku-sub {
  @include md-body-small;
  color: var(--md-on-surface-variant);
  margin: 4px 0;
}

.sku-price {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.sku-price-plain {
  @include md-title-medium;
  font-weight: 700;
  color: var(--md-primary);
}

.np {
  @include md-title-medium;
  font-weight: 700;
  color: var(--md-primary);
}

.strike {
  @include md-label-small;
  color: var(--md-outline);
  text-decoration: line-through;
}

.np-tag {
  @include md-label-small;
  background: var(--md-tertiary-container);
  color: var(--md-on-tertiary-container);
  border-radius: $md-shape-xs;
  padding: 1px 6px;

  &.is-promo {
    background: var(--md-error-container);
    color: var(--md-on-error-container);
  }
}

// ---- 优惠券 ----
.coupon-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid var(--md-outline-variant);
  cursor: pointer;

  &:active {
    opacity: 0.7;
  }
}

.c-label {
  @include md-body-medium;
  color: var(--md-on-surface-variant);
}

.c-value {
  flex: 1;
  min-width: 0;
  @include md-body-small;
  color: var(--md-primary);
}

.c-go {
  @include md-label-medium;
  color: var(--md-tertiary);
}

.c-arrow {
  color: var(--md-on-surface-variant);
}

.promo-count {
  margin-top: 12px;
  background: var(--md-error-container);
  color: var(--md-on-error-container);
  border-radius: $md-shape-sm;
  padding: 6px 12px;
  @include md-body-small;
  font-weight: 500;
}

.np-tip {
  margin-top: 12px;
  background: var(--md-tertiary-container);
  color: var(--md-on-tertiary-container);
  border-radius: $md-shape-sm;
  padding: 8px 12px;
  @include md-body-small;
}

// ---- 拼团 ----
.gb-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 16px;
  padding: 12px;
  background: var(--md-surface-container-highest);
  border-radius: $md-shape-md;
}

.gb-info {
  flex: 1;
  min-width: 0;
}

.gb-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.gb-price {
  @include md-title-medium;
  font-weight: 700;
  color: var(--md-primary);
}

.gb-act {
  @include md-label-small;
  color: var(--md-on-error-container);
  border: 1px solid var(--md-error);
  border-radius: $md-shape-xs;
  padding: 0 6px;
}

.gb-tip {
  display: block;
  margin-top: 4px;
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

// ---- 大纲 ----
.rich {
  @include md-body-medium;
  color: var(--md-on-surface-variant);
  line-height: 1.7;
  white-space: pre-wrap;
}

// ---- 教练 ----
.coach-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
  cursor: pointer;

  &:active {
    opacity: 0.7;
  }
}

.coach-info {
  flex: 1;
  min-width: 0;
}

.coach-name {
  display: flex;
  align-items: center;
  gap: 8px;
  @include md-title-small;
  color: var(--md-on-surface);
}

.cert {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  @include md-label-small;
  color: var(--md-tertiary);
  background: var(--md-tertiary-container);
  border-radius: $md-shape-xs;
  padding: 1px 6px;
}

.coach-sub {
  @include md-body-small;
  color: var(--md-on-surface-variant);
  margin-top: 2px;
}

.coach-arrow {
  color: var(--md-on-surface-variant);
}

// ---- 评价 / 问答 ----
.tab-body {
  padding-top: 4px;
}

.review {
  padding: 12px 0;
  border-bottom: 1px solid var(--md-outline-variant);
}

.review-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.review-time {
  @include md-label-small;
  color: var(--md-on-surface-variant);
}

.review-content {
  @include md-body-medium;
  color: var(--md-on-surface);
  margin-top: 6px;
  line-height: 1.6;
}

.review-reply {
  margin-top: 8px;
  background: var(--md-surface-container);
  border-radius: $md-shape-sm;
  padding: 8px 12px;
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

.all-reviews {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 2px;
  padding: 14px 0 2px;
  @include md-label-large;
  color: var(--md-primary);
  cursor: pointer;

  &:active {
    opacity: 0.7;
  }
}

.empty-inline {
  @include md-body-small;
  color: var(--md-outline);
  text-align: center;
  padding: 16px 0;
}

.qa-ask-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid var(--md-outline-variant);
  cursor: pointer;

  &:active {
    opacity: 0.7;
  }
}

.qa-ask-tip {
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

.qa-ask-icon {
  color: var(--md-on-surface-variant);
}

.qa-form {
  background: var(--md-surface-container-highest);
  border-radius: $md-shape-md;
  padding: 12px;
  margin: 12px 0;
}

.qa-form-foot {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 10px;
}

.qa-anon {
  display: flex;
  align-items: center;
  gap: 8px;
}

.qa-anon-label {
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

.qa-item {
  padding: 12px 0;
  border-bottom: 1px solid var(--md-outline-variant);

  &:last-child {
    border-bottom: none;
  }
}

.qa-row {
  display: flex;
  gap: 8px;
}

.qa-answer {
  margin-top: 10px;
  margin-left: 8px;
}

.qa-badge {
  flex-shrink: 0;
  min-width: 24px;
  height: 24px;
  border-radius: $md-shape-xs;
  text-align: center;
  line-height: 24px;
  padding: 0 4px;
  @include md-label-small;

  &.is-q {
    background: var(--md-primary-container);
    color: var(--md-on-primary-container);
  }
  &.is-a {
    background: var(--md-secondary-container);
    color: var(--md-on-secondary-container);
  }
  &.is-coach {
    background: var(--md-primary);
    color: var(--md-on-primary);
  }
}

.qa-body {
  flex: 1;
  min-width: 0;
}

.qa-content {
  @include md-body-medium;
  color: var(--md-on-surface);
  font-weight: 500;

  &.is-answer {
    font-weight: 400;
    color: var(--md-on-surface-variant);
    line-height: 1.6;
  }
}

.qa-meta {
  @include md-label-small;
  color: var(--md-outline);
  margin-top: 4px;
}

.qa-pin {
  flex-shrink: 0;
  @include md-label-small;
  color: var(--md-on-error-container);
  border: 1px solid var(--md-error);
  border-radius: $md-shape-xs;
  padding: 0 6px;
  height: 18px;
  line-height: 18px;
}

.qa-none {
  margin-top: 8px;
  @include md-body-small;
  color: var(--md-outline);
}

// ---- 底部操作栏 ----
.footer-space {
  height: 84px;
}

.footer-bar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 90;
  background: var(--md-surface-container);
  padding: 10px 16px calc(10px + env(safe-area-inset-bottom));
  @include md-elevation-2;
}

.footer-inner {
  display: flex;
  align-items: center;
  gap: 16px;
}

.pay-amount {
  flex: 1;
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.pa-label {
  @include md-body-small;
  color: var(--md-on-surface-variant);
}

.pa-num {
  @include md-headline-small;
  font-weight: 700;
  color: var(--md-primary);
}

.buy-btn {
  flex: 1.2;
}

.success-text {
  @include md-body-medium;
  color: var(--md-on-surface);
  line-height: 1.6;
}

.skeleton-gap {
  height: 12px;
}
</style>
