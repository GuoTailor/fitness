// 环境配置：后端地址按端区分
// - H5 开发走 vite 代理（空串=同源 /api → 127.0.0.1:8080）；生产默认同源（nginx 反代），
//   独立域名部署时用 VITE_API_BASE 覆盖
// - 小程序直连后端（上线需在微信后台配置合法域名）
export const API_BASE = (() => {
  // #ifdef MP-WEIXIN
  return 'http://127.0.0.1:8080'
  // #endif
  // #ifdef H5
  // eslint-disable-next-line no-undef
  return (typeof import.meta !== 'undefined' && import.meta.env && import.meta.env.VITE_API_BASE) || ''
  // #endif
})()

// 是否为开发模式（H5 显示 mock 身份登录入口等）
export const IS_DEV = (() => {
  // #ifdef H5
  return import.meta.env.DEV
  // #endif
  // #ifdef MP-WEIXIN
  return false
  // #endif
})()
