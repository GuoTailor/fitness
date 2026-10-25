# client 页面移植规范（Material Design 3）

本文档供页面实现（人或 AI 代理）遵循。目标：把 `../miniprogram/pages/` 的原生小程序页面重写为本工程的 Vue3 uni-app 页面，UI 全部按 MD3 重做。

## 工程约定

- 框架：uni-app Vue3 `<script setup>`，SCSS。双端：H5 + 微信小程序。
- 组件免注册：`M3Xxx` 组件经 easycom 自动引入，直接用 `<M3Button>` 等。
- 单位：一律 **px**（不用 rpx）。颜色一律用 CSS 变量（`var(--md-primary)` 等），**禁止硬编码色值**。字型/圆角/阴影用 uni.scss 的 mixin：`@include md-title-medium; @include md-elevation-1;`（组件 `<style lang="scss" scoped>` 内直接可用，无需 import）。
- 布局：页面根 `class="md-page"`；内容壳 `class="md-shell"`（限宽 640px 居中）。三个 tab 页（index/course/list/mine）底部放 `<M3BottomNavigation>`（组件自带占位）。
- 图标名必须是 `scripts/build_icon_font.mjs` ICONS 清单内的（Material Icons Round 命名），缺了先在脚本 ICONS 数组加名字并运行 `node scripts/build_icon_font.mjs`。

## 请求层（替代原 wx.request 封装）

```js
import { request, uploadImage } from '@/utils/request'
const data = await request({ url: '/api/v1/packages', data: { page: 1 } })       // GET
await request({ url: '/api/v1/orders', method: 'POST', data: { skuId } })        // POST
await uploadImage(filePath)                                                      // 上传，返回 {url}
```

- **路径必须带 `/api/v1` 前缀**（原小程序代码里也是 `/api/v1/...`，照抄即可）。
- 后端契约 `{code,msg,data}`；`code!==0` 请求层已统一 toast + reject，页面 catch 里一般无需再 toast。
- `silent: true` 不弹错误（轮询/静默请求用）。
- 登录态：页面 onLoad 里 `await auth.ensureLogin()`（需登录的页面）；token 失效自动清理/跳登录。

## 鉴权 store（替代原 app.globalData / utils/auth）

```js
import { useAuthStore } from '@/stores/auth'
const auth = useAuthStore()
await auth.ensureLogin()        // MP 静默登录 / H5 未登录跳登录页(并 throw)
auth.user.id                    // 当前用户
auth.isCoach / auth.coachId
```

## 支付（统一用 payOrder，不要自己写 prepay）

```js
import { payOrder } from '@/utils/pay'
// H5 需要传 onQrCode 接收二维码 dataURL 并弹出 <M3PaySheet>
const res = await payOrder(orderId, {
  onQrCode: (dataUrl) => { qr.value = dataUrl; payOpen.value = true }
})
if (res.paid) { /* 支付成功 */ }
```

## 平台差异（条件编译写法）

```js
// #ifdef MP-WEIXIN
...wx 专属逻辑（订阅消息 uni.requestSubscribeMessage、open-type 按钮等）
// #endif
// #ifdef H5
...网页逻辑（复制分享链接 shareLink()、文件选择等）
// #endif
```

规则：
- `wx.xxx` → `uni.xxx`（showToast/showModal/navigateTo/switchTab→reLaunch 等）。
- 原页面 `getApp().globalData.xxx` → auth store；`app.ensureLogin()` → `auth.ensureLogin()`。
- 原页面 url 跳转：普通页 navigateTo；**tab 页（index/course/list/mine）用 `uni.reLaunch({url})`**（无原生 tabBar）。
- 分享：MP 保留 `onShareAppMessage`（配合 `mpSharePayload(path, auth.user?.id)`，from '@/utils/referral'）；H5 用 `shareLink(path, uid)`（复制链接）。
- 订阅消息（utils/subscribe.js 场景）：仅 MP 包裹在 `// #ifdef MP-WEIXIN`。
- 老带新捕获（原 index 页 captureInvite）：已上移到 App.vue onLaunch，页面不用管；绑定触发（tryBindInvite）已在登录成功后自动执行，页面不用管。
- 头像选择：MP 用 `open-type="chooseAvatar"` 按钮；H5 用 `<input type="file">`（包装在 label 里，onchange 读临时路径 → uploadImage）。H5 的 chooseImage 用 `uni.chooseImage` 也可（uni-app H5 已适配）。
- 客服/手机号快捷授权按钮：MP 保留 open-type；H5 隐藏（客服改静态弹层）。

## 页面骨架模板

普通页（含返回栏）：

```vue
<template>
  <view class="md-page">
    <M3TopAppBar title="页面标题" :back="true" :elevated="scrolled" />
    <view class="md-shell page-body">
      <!-- 内容 -->
    </view>
  </view>
</template>
<script setup>
import { ref } from 'vue'
import { onPageScroll, onPullDownRefresh } from '@dcloudio/uni-app'
const scrolled = ref(false)
onPageScroll((e) => { scrolled.value = e.scrollTop > 8 })
</script>
<style lang="scss" scoped>
.page-body { padding: 12px 16px 32px; }
</style>
```

tab 页（无 back；底部导航）：

```vue
<template>
  <view class="md-page">
    <M3TopAppBar title="首页" />
    <view class="md-shell page-body">...</view>
    <M3BottomNavigation :items="TABS" v-model="'home'" @change="onTab" />
  </view>
</template>
<script setup>
const TABS = [
  { key: 'home', icon: 'home', label: '首页' },
  { key: 'course', icon: 'shopping_bag', label: '课包' },
  { key: 'mine', icon: 'person', label: '我的' }
]
function onTab(t) {
  uni.reLaunch({ url: t.key === 'home' ? '/pages/index/index' : t.key === 'course' ? '/pages/course/list' : '/pages/mine/mine' })
}
</script>
```

## 组件速查（props/事件）

- `M3Button`：variant(filled/tonal/outlined/text/elevated) size(sm/md/lg) icon block loading disabled danger；@click
- `M3IconButton`：icon size；@click
- `M3Card`：variant(elevated/filled/outlined) clickable；@click；默认插槽
- `M3TopAppBar`：title back elevated；插槽 leading/actions
- `M3BottomNavigation`：items v-model；@change
- `M3TextField`：v-model label placeholder type(text/number/password) multiline maxlength disabled error hint labelBg；插槽 trail；@confirm
- `M3Dialog`：open(title, icon, dismissible)；@close；插槽 default/actions
- `M3BottomSheet`：open title；@close；默认插槽
- `M3Chip`：label selected filter closable variant(outlined/tonal)；@click @close
- `M3ListItem`：headline supporting trailingIcon clickable；插槽 leading/trailing/default；@click
- `M3Stars`：v-model size editable showValue
- `M3Avatar`：src name size
- `M3EmptyState`：icon title description；默认插槽放按钮
- `M3Switch` v-model；`M3Progress`(type linear/circular, v-model, indeterminate)；`M3Skeleton`(variant text/rect/circle width height)；`M3Divider`(inset)；`M3SegmentedButton`(options[{value,label,icon}] v-model)；`M3Tabs`(tabs[{value,label}] v-model)；`M3Fab`(icon label)；@click；`M3Badge`(count dot)；`M3PaySheet`(open qr amount hint)；@close

## 设计语言要点（照 m3.material.io）

- 表面层次：页面底 `--md-surface`；卡片用 filled(surface-container-high) 或 outlined；弹层 surface-container-high；导航条 surface-container。
- 主操作按钮页面内唯一（filled），次操作 tonal/outlined/text；破坏性操作 danger + M3Dialog 确认。
- 状态用 chip 筛选行（横向滚动）、订单/预约状态用「颜色圆点 + label-small」而非大色块。
- 金额一律 `--md-primary` 加粗（utils/format.js 的 yuan()）。
- 列表页骨架屏（M3Skeleton 卡片 x3）→ 空态（M3EmptyState）→ 数据三态齐全。
- 每个可点区域 :active 有按压反馈（组件已内置；自定义 view 需自加 :active 背景变化）。

## 验证

页面写完必须跑：
```
cd client && npm run build:h5 && npm run build:mp-weixin
```
两构建零错误才算完成。不要起 dev server、不要提交 git。
