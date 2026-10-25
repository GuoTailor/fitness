<template>
  <view class="md-page">
    <M3TopAppBar title="我的" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 用户卡片 / 编辑资料 -->
      <template v-if="auth.user">
        <!-- 编辑态 -->
        <M3Card v-if="editing" variant="filled" class="edit-card">
          <view class="edit-title">编辑资料</view>
          <view class="edit-avatar-row">
            <!-- MP：微信头像快捷选择 -->
            <!-- #ifdef MP-WEIXIN -->
            <button class="avatar-btn" open-type="chooseAvatar" @chooseavatar="onChooseAvatar">
              <M3Avatar :src="editAvatar || auth.user.avatarUrl" :name="auth.user.nickname" :size="64" />
              <text class="avatar-tip">点击换头像</text>
            </button>
            <!-- #endif -->
            <!-- H5：相册选图上传 -->
            <!-- #ifdef H5 -->
            <view class="avatar-btn" @click="onChooseAvatarH5">
              <M3Avatar :src="editAvatar || auth.user.avatarUrl" :name="auth.user.nickname" :size="64" />
              <text class="avatar-tip">点击换头像</text>
            </view>
            <!-- #endif -->
          </view>
          <M3TextField
            v-model="editNickname"
            label="昵称"
            placeholder="请输入昵称"
            :maxlength="64"
            label-bg="var(--md-surface-container-high)"
          />
          <view class="edit-btns">
            <M3Button variant="outlined" @click="editing = false">取消</M3Button>
            <M3Button variant="filled" :loading="savingProfile" @click="onSaveProfile">保存</M3Button>
          </view>
        </M3Card>

        <!-- 展示态 -->
        <M3Card v-else variant="filled" class="profile-card">
          <view class="profile-main">
            <M3Avatar :src="auth.user.avatarUrl" :name="auth.user.nickname" :size="56" />
            <view class="profile-info">
              <text class="profile-name md-ellipsis">{{ auth.user.nickname || '学员' }}</text>
              <text class="profile-sub">{{ phoneText }}</text>
            </view>
            <M3Button variant="text" icon="edit" size="sm" @click="onEditProfile">编辑</M3Button>
          </view>
        </M3Card>

        <!-- 老带新 -->
        <M3Card variant="outlined" class="ref-card">
          <view class="ref-inner">
            <view class="ref-info">
              <text class="ref-title">邀请好友得课包券</text>
              <text class="ref-sub">{{ referralText }}</text>
            </view>
            <!-- MP：转发卡片 -->
            <!-- #ifdef MP-WEIXIN -->
            <button class="pill-btn" open-type="share">立即邀请</button>
            <!-- #endif -->
            <!-- H5：复制/系统分享链接 -->
            <!-- #ifdef H5 -->
            <M3Button variant="tonal" size="sm" icon="share" @click="onInvite">立即邀请</M3Button>
            <!-- #endif -->
          </view>
        </M3Card>

        <!-- 常用入口 -->
        <M3Card variant="outlined" class="menu-card">
          <M3ListItem headline="我的订单" clickable @click="go('/pages/order/list')">
            <template #leading><M3Icon name="receipt_long" :size="22" /></template>
          </M3ListItem>
          <M3Divider inset />
          <M3ListItem headline="我的课包" clickable @click="go('/pages/training/index')">
            <template #leading><M3Icon name="shopping_bag" :size="22" /></template>
          </M3ListItem>
          <M3Divider inset />
          <M3ListItem headline="我的预约" clickable @click="go('/pages/booking/list')">
            <template #leading><M3Icon name="event_available" :size="22" /></template>
          </M3ListItem>
          <M3Divider inset />
          <M3ListItem headline="我的评价" clickable @click="go('/pages/review/list')">
            <template #leading><M3Icon name="reviews" :size="22" /></template>
          </M3ListItem>
          <M3Divider inset />
          <M3ListItem headline="我的优惠券" clickable @click="go('/pages/coupon/mine')">
            <template #leading><M3Icon name="confirmation_number" :size="22" /></template>
          </M3ListItem>
          <M3Divider inset />
          <M3ListItem headline="领券中心" clickable @click="go('/pages/coupon/center')">
            <template #leading><M3Icon name="redeem" :size="22" /></template>
          </M3ListItem>
          <M3Divider inset />
          <M3ListItem headline="我的提问" clickable @click="go('/pages/qa/mine')">
            <template #leading><M3Icon name="question_answer" :size="22" /></template>
          </M3ListItem>

          <!-- 绑定手机号：仅 MP（H5 注册时已有手机号，不提供绑定入口） -->
          <!-- #ifdef MP-WEIXIN -->
          <template v-if="!auth.user.phone">
            <M3Divider inset />
            <button
              class="menu-native-btn"
              open-type="getPhoneNumber"
              :loading="phoneBinding"
              @getphonenumber="onGetPhone"
            >
              <M3Icon name="badge" :size="22" class="menu-icon" />
              <text class="menu-native-label">绑定手机号</text>
              <M3Icon name="chevron_right" :size="24" class="menu-chevron" />
            </button>
          </template>
          <!-- #endif -->

          <M3Divider inset />
          <!-- 联系客服：MP 会话按钮；H5 静态信息弹层 -->
          <!-- #ifdef MP-WEIXIN -->
          <button class="menu-native-btn" open-type="contact">
            <M3Icon name="support_agent" :size="22" class="menu-icon" />
            <text class="menu-native-label">联系客服</text>
            <M3Icon name="chevron_right" :size="24" class="menu-chevron" />
          </button>
          <!-- #endif -->
          <!-- #ifdef H5 -->
          <M3ListItem headline="联系客服" clickable @click="contactOpen = true">
            <template #leading><M3Icon name="support_agent" :size="22" /></template>
          </M3ListItem>
          <!-- #endif -->

          <M3Divider inset />
          <M3ListItem v-if="auth.isCoach" headline="教练工作台" clickable @click="go('/pages/coach-admin/index')">
            <template #leading><M3Icon name="manage_accounts" :size="22" /></template>
          </M3ListItem>
          <M3ListItem v-else headline="教练入驻" clickable @click="go('/pages/coach/apply')">
            <template #leading><M3Icon name="school" :size="22" /></template>
          </M3ListItem>
          <M3Divider inset />
          <M3ListItem headline="用户协议" clickable @click="go('/pages/agreement/agreement?type=user')">
            <template #leading><M3Icon name="description" :size="22" /></template>
          </M3ListItem>
          <M3Divider inset />
          <M3ListItem class="logout-item" headline="退出登录" clickable @click="logoutOpen = true">
            <template #leading><M3Icon name="logout" :size="22" /></template>
          </M3ListItem>
        </M3Card>

        <view class="links">
          <text class="link" @click="go('/pages/agreement/agreement?type=privacy')">隐私政策</text>
          <text class="dot">·</text>
          <text class="link" @click="go('/pages/agreement/agreement?type=user')">用户协议</text>
        </view>
      </template>

      <!-- 未登录 -->
      <M3Card v-else variant="filled" class="login-card">
        <M3EmptyState icon="person" title="未登录" description="登录后可购买课包、预约教练">
          <M3Button variant="filled" icon="login" @click="onLogin">登录</M3Button>
        </M3EmptyState>
      </M3Card>
    </view>

    <!-- 联系客服（H5 静态信息） -->
    <!-- #ifdef H5 -->
    <M3Dialog :open="contactOpen" title="联系客服" icon="support_agent" @close="contactOpen = false">
      <view class="contact-list">
        <view class="contact-row">
          <M3Icon name="call" :size="20" class="contact-icon" />
          <text class="contact-text">客服电话：400-000-0000</text>
        </view>
        <view class="contact-row">
          <M3Icon name="chat_bubble" :size="20" class="contact-icon" />
          <text class="contact-text">微信客服：fitness-studio</text>
        </view>
        <text class="contact-sub">工作时间 9:00-21:00，也可到店咨询</text>
      </view>
      <template #actions>
        <M3Button variant="filled" @click="contactOpen = false">知道了</M3Button>
      </template>
    </M3Dialog>
    <!-- #endif -->

    <!-- 退出登录确认 -->
    <M3Dialog :open="logoutOpen" title="退出登录" icon="logout" :dismissible="false">
      <text>退出后将无法查看课包与预约进度，确定退出？</text>
      <template #actions>
        <M3Button variant="text" @click="logoutOpen = false">取消</M3Button>
        <M3Button variant="filled" danger @click="doLogout">退出</M3Button>
      </template>
    </M3Dialog>

    <M3BottomNavigation :items="TABS" model-value="mine" @change="onTab" />
  </view>
</template>

<script setup>
import { ref, computed } from 'vue'
import { onLoad, onShow, onPageScroll, onShareAppMessage } from '@dcloudio/uni-app'
import { request, uploadImage } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
import { maskPhone } from '@/utils/format'
import { mpSharePayload, shareLink } from '@/utils/referral'

const auth = useAuthStore()

const TABS = [
  { key: 'home', icon: 'home', label: '首页' },
  { key: 'course', icon: 'shopping_bag', label: '课包' },
  { key: 'mine', icon: 'person', label: '我的' }
]

const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })

// ---- 数据 ----
const referral = ref({ bound: false, rewarded: false })
const editing = ref(false)
const editNickname = ref('')
const editAvatar = ref('')
const savingProfile = ref(false)
const phoneBinding = ref(false)
const contactOpen = ref(false)
const logoutOpen = ref(false)

const phoneText = computed(() =>
  auth.user && auth.user.phone ? maskPhone(auth.user.phone) : '未绑定手机号'
)

const referralText = computed(() => {
  if (!referral.value.bound) return '新客首单双方各得奖励'
  return referral.value.rewarded ? '好友已下单，奖励已到账' : '已接受好友邀请'
})

onLoad(async () => {
  await auth.ensureLogin().catch(() => {})
})

onShow(() => {
  if (!auth.token) return
  auth.refreshMe()
  loadReferral()
})

async function loadReferral() {
  try {
    referral.value = await request({ url: '/api/v1/referral/mine' })
  } catch (e) { /* 弱网容忍 */ }
}

// ---- tab 切换（无原生 tabBar，统一 reLaunch）----
function onTab(t) {
  uni.reLaunch({
    url: t.key === 'home' ? '/pages/index/index' : t.key === 'course' ? '/pages/course/list' : '/pages/mine/mine'
  })
}

// ---- 登录 ----
async function onLogin() {
  await auth.ensureLogin().catch(() => {})
  if (auth.token) {
    await auth.refreshMe()
    loadReferral()
  }
}

// ---- 编辑资料 ----
function onEditProfile() {
  editNickname.value = (auth.user && auth.user.nickname) || ''
  editAvatar.value = ''
  editing.value = true
}

async function onSaveProfile() {
  const nickname = editNickname.value.trim()
  if (!nickname) {
    uni.showToast({ title: '请填写昵称', icon: 'none' })
    return
  }
  savingProfile.value = true
  try {
    await request({
      url: '/api/v1/me',
      method: 'PUT',
      data: { nickname, avatarUrl: editAvatar.value || '' }
    })
    await auth.refreshMe()
    editing.value = false
    uni.showToast({ title: '已保存', icon: 'success' })
  } catch (e) { /* toast 已处理 */ } finally {
    savingProfile.value = false
  }
}

// 头像：MP 走 chooseAvatar 临时路径 → 站内上传
// #ifdef MP-WEIXIN
async function onChooseAvatar(e) {
  const path = e && e.detail && e.detail.avatarUrl
  if (!path) return
  uni.showLoading({ title: '上传中…' })
  try {
    const res = await uploadImage(path)
    editAvatar.value = (res && res.url) || ''
  } catch (err) { /* toast 已处理 */ } finally {
    uni.hideLoading()
  }
}
// #endif

// 头像：H5 用 uni.chooseImage 选图 → 站内上传
// #ifdef H5
function onChooseAvatarH5() {
  uni.chooseImage({
    count: 1,
    success: async (res) => {
      const path = res.tempFilePaths && res.tempFilePaths[0]
      if (!path) return
      uni.showLoading({ title: '上传中…' })
      try {
        const url = await uploadImage(path)
        editAvatar.value = (url && url.url) || ''
      } catch (err) { /* toast 已处理 */ } finally {
        uni.hideLoading()
      }
    }
  })
}
// #endif

// ---- 手机号快捷绑定（仅 MP；H5 注册时已有手机号，不展示入口）----
// #ifdef MP-WEIXIN
async function onGetPhone(e) {
  const code = e && e.detail && e.detail.code
  if (!code) return
  phoneBinding.value = true
  try {
    await request({ url: '/api/v1/auth/phone', method: 'POST', data: { code } })
    await auth.refreshMe()
    uni.showToast({ title: '绑定成功', icon: 'success' })
  } catch (err) { /* toast 已处理 */ } finally {
    phoneBinding.value = false
  }
}
// #endif

// ---- 老带新分享 ----
// #ifdef MP-WEIXIN
onShareAppMessage(() => mpSharePayload('/pages/index/index', auth.user && auth.user.id))
// #endif

// #ifdef H5
function onInvite() {
  shareLink('/pages/index/index', auth.user && auth.user.id)
}
// #endif

// ---- 退出登录 ----
function doLogout() {
  logoutOpen.value = false
  auth.logout()
  // H5 退出后跳登录页；MP 静默登录体系下留在本页
  // #ifdef H5
  uni.reLaunch({ url: '/pages/login/login' })
  // #endif
}

function go(url) {
  uni.navigateTo({ url })
}
</script>

<style lang="scss" scoped>
.page-body {
  padding: 12px 16px 32px;
}

.profile-card,
.edit-card,
.ref-card,
.menu-card,
.login-card {
  margin-bottom: 12px;
}

// ---- 用户卡片 ----
.profile-main {
  padding: 16px;
  display: flex;
  align-items: center;
  gap: 14px;
}
.profile-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.profile-name {
  color: var(--md-on-surface);
  @include md-title-large;
}
.profile-sub {
  color: var(--md-on-surface-variant);
  @include md-body-medium;
}

// ---- 编辑资料 ----
.edit-card {
  padding: 16px;
}
.edit-title {
  color: var(--md-on-surface);
  @include md-title-medium;
  margin-bottom: 16px;
}
.edit-avatar-row {
  display: flex;
  justify-content: center;
  margin-bottom: 20px;
}
.avatar-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  margin: 0;
  background: transparent;
  border: none;
  border-radius: $md-shape-md;
  line-height: inherit;
  font-size: inherit;
  &::after {
    border: none;
  }
  &:active {
    background: rgba(0, 0, 0, $md-state-pressed-opacity);
  }
}
.avatar-tip {
  color: var(--md-on-surface-variant);
  @include md-label-medium;
}
.edit-btns {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

// ---- 老带新 ----
.ref-inner {
  padding: 14px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
}
.ref-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.ref-title {
  color: var(--md-on-surface);
  @include md-title-medium;
}
.ref-sub {
  color: var(--md-on-surface-variant);
  @include md-body-small;
}
.pill-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 32px;
  padding: 0 16px;
  margin: 0;
  border: none;
  border-radius: $md-shape-full;
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);
  font-size: 12px;
  font-weight: 500;
  line-height: 1;
  flex-shrink: 0;
  &::after {
    border: none;
  }
  &:active {
    opacity: 0.88;
  }
}

// ---- 菜单 ----
.menu-native-btn {
  display: flex;
  align-items: center;
  gap: 16px;
  width: 100%;
  min-height: 56px;
  padding: 12px 16px;
  margin: 0;
  background: transparent;
  border: none;
  border-radius: 0;
  text-align: left;
  line-height: inherit;
  font-size: inherit;
  box-sizing: border-box;
  &::after {
    border: none;
  }
  &:active {
    background: rgba(0, 0, 0, $md-state-pressed-opacity);
  }
}
.menu-native-label {
  flex: 1;
  min-width: 0;
  color: var(--md-on-surface);
  @include md-title-medium;
}
.menu-icon {
  color: var(--md-on-surface-variant);
  flex-shrink: 0;
}
.menu-chevron {
  color: var(--md-on-surface-variant);
  flex-shrink: 0;
}
.logout-item {
  :deep(.m3-li__headline) {
    color: var(--md-error);
  }
}

// ---- 底部协议链接 ----
.links {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 8px;
  padding: 4px 0 8px;
}
.link {
  color: var(--md-on-surface-variant);
  @include md-body-small;
  text-decoration: underline;
  &:active {
    color: var(--md-primary);
  }
}
.dot {
  color: var(--md-outline);
}

// ---- 客服弹层（H5）----
.contact-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.contact-row {
  display: flex;
  align-items: center;
  gap: 10px;
}
.contact-icon {
  color: var(--md-primary);
  flex-shrink: 0;
}
.contact-text {
  color: var(--md-on-surface);
  @include md-body-large;
}
.contact-sub {
  color: var(--md-on-surface-variant);
  @include md-body-small;
}
</style>
