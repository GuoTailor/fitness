<script>
import { useAuthStore } from './stores/auth'
import { captureInvite } from './utils/referral'

export default {
  onLaunch(options) {
    // 鉴权初始化：已有 token 静默刷新用户信息；注册 401 兜底清理
    useAuthStore().init()
    // 老带新邀请落地：H5 从 URL query、小程序从启动参数捕获 inviter
    captureInvite(options)
  }
}
</script>

<style lang="scss">
@import './styles/main.scss';
</style>
