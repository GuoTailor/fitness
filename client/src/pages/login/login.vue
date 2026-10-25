<template>
  <view class="md-page">
    <view class="login md-shell">
      <!-- 品牌区 -->
      <view class="login__brand">
        <view class="login__logo">
          <M3Icon name="fitness_center" :size="32" />
        </view>
        <text class="login__title">健身工作室</text>
        <text class="login__subtitle">课包 · 约课 · 训练打卡</text>
      </view>

      <!-- H5：手机号 + 验证码 -->
      <!-- #ifdef H5 -->
      <view class="login__form">
        <M3TextField v-model="phone" label="手机号" type="number" :maxlength="11" placeholder="请输入手机号" />
        <view class="login__code-row">
          <view class="login__code-input">
            <M3TextField v-model="smsCode" label="验证码" type="number" :maxlength="6" placeholder="6 位验证码" @confirm="onLogin" />
          </view>
          <M3Button variant="tonal" size="lg" :disabled="countdown > 0 || !phoneValid" @click="onSendSms">
            {{ countdown > 0 ? countdown + 's' : '获取验证码' }}
          </M3Button>
        </view>
        <M3Button block size="lg" :loading="submitting" @click="onLogin">登 录</M3Button>
        <view class="login__agreement">
          <text>登录即代表同意</text>
          <text class="login__link" @click="goAgreement('user')">《用户协议》</text>
          <text>与</text>
          <text class="login__link" @click="goAgreement('privacy')">《隐私政策》</text>
        </view>
      </view>
      <!-- #endif -->

      <!-- MP：微信一键登录 -->
      <!-- #ifdef MP-WEIXIN -->
      <view class="login__form">
        <M3Button block size="lg" :loading="submitting" @click="onWxLogin">微信一键登录</M3Button>
        <view class="login__agreement">
          <text>登录即代表同意</text>
          <text class="login__link" @click="goAgreement('user')">《用户协议》</text>
          <text>与</text>
          <text class="login__link" @click="goAgreement('privacy')">《隐私政策》</text>
        </view>
      </view>
      <!-- #endif -->

      <!-- 开发模式：mock 身份（后端 wechat.mock=true 才可用） -->
      <view v-if="isDev" class="login__dev">
        <M3Divider />
        <text class="login__dev-title">开发模式：指定 mock 身份登录</text>
        <view class="login__dev-row">
          <view class="login__dev-input">
            <M3TextField v-model="devCode" label="身份码" placeholder="mock-user-1 / mock-coach-1" />
          </view>
          <M3Button variant="outlined" size="lg" :loading="devSubmitting" @click="onDevLogin">登录</M3Button>
        </view>
      </view>
    </view>
  </view>
</template>

<script setup>
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { useAuthStore } from '@/stores/auth'
import { request } from '@/utils/request'
import { IS_DEV as isDev } from '@/utils/config'

const auth = useAuthStore()
const phone = ref('')
const smsCode = ref('')
const countdown = ref(0)
const submitting = ref(false)
const devCode = ref('')
const devSubmitting = ref(false)
let redirect = ''
let timer = null

const phoneValid = computed(() => /^1\d{10}$/.test(phone.value))

onLoad((opts) => {
  redirect = opts && opts.redirect ? decodeURIComponent(opts.redirect) : ''
  // 已登录直接离开登录页
  if (auth.token) {
    goAfterLogin()
  }
})

function goAfterLogin() {
  if (redirect && redirect !== '/pages/login/login') {
    uni.reLaunch({ url: redirect })
  } else {
    uni.reLaunch({ url: '/pages/index/index' })
  }
}

async function onSendSms() {
  if (!phoneValid.value || countdown.value > 0) return
  const res = await request({ url: '/api/v1/auth/sms/send', method: 'POST', data: { phone: phone.value } })
  if (res.devCode) {
    uni.showModal({ title: '开发模式验证码', content: res.devCode, showCancel: false })
  } else {
    uni.showToast({ title: '验证码已发送', icon: 'none' })
  }
  countdown.value = 60
  timer = setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0) clearInterval(timer)
  }, 1000)
}

async function onLogin() {
  if (!phoneValid.value) {
    uni.showToast({ title: '请输入正确的手机号', icon: 'none' })
    return
  }
  if (!smsCode.value || submitting.value) return
  submitting.value = true
  try {
    const res = await request({
      url: '/api/v1/auth/login/phone',
      method: 'POST',
      data: { phone: phone.value, smsCode: smsCode.value }
    })
    auth.applyLoginResult(res)
    uni.showToast({ title: '登录成功', icon: 'none' })
    goAfterLogin()
  } catch (e) {
    // 错误提示由请求层统一 toast
  } finally {
    submitting.value = false
  }
}

async function onWxLogin() {
  submitting.value = true
  try {
    await auth.loginByWechat()
    goAfterLogin()
  } catch (e) {
    // toast 已提示
  } finally {
    submitting.value = false
  }
}

async function onDevLogin() {
  if (!devCode.value || devSubmitting.value) return
  devSubmitting.value = true
  try {
    await auth.loginByWechat(devCode.value)
    goAfterLogin()
  } catch (e) {
    // toast 已提示
  } finally {
    devSubmitting.value = false
  }
}

function goAgreement(type) {
  uni.navigateTo({ url: `/pages/agreement/agreement?type=${type}` })
}
</script>

<style lang="scss" scoped>
.login {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  padding: 0 24px calc(24px + env(safe-area-inset-bottom));

  &__brand {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 64px 0 40px;
  }
  &__logo {
    width: 72px;
    height: 72px;
    border-radius: $md-shape-xl;
    background: var(--md-primary-container);
    color: var(--md-on-primary-container);
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 16px;
  }
  &__title {
    @include md-headline-small;
    color: var(--md-on-surface);
  }
  &__subtitle {
    margin-top: 4px;
    color: var(--md-on-surface-variant);
    @include md-body-medium;
  }

  &__form {
    display: flex;
    flex-direction: column;
    gap: 16px;
    margin-top: 8px;
  }
  &__code-row {
    display: flex;
    gap: 12px;
    align-items: flex-start;
  }
  &__code-input {
    flex: 1;
    min-width: 0;
  }
  &__agreement {
    display: flex;
    justify-content: center;
    flex-wrap: wrap;
    margin-top: 8px;
    color: var(--md-on-surface-variant);
    @include md-body-small;
  }
  &__link {
    color: var(--md-primary);
  }

  &__dev {
    margin-top: 40px;
    &-title {
      display: block;
      margin: 16px 0 12px;
      color: var(--md-tertiary);
      @include md-title-small;
    }
    &-row {
      display: flex;
      gap: 12px;
      align-items: flex-start;
    }
    &-input {
      flex: 1;
      min-width: 0;
    }
  }
}
</style>
