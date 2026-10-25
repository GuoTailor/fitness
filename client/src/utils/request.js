// 网络层：uni.request Promise 封装
// 契约：后端统一 {code, msg, data}，code!==0 时 toast 并 reject；
// 401/40101 清除本地登录态并通知 auth store 跳登录
import { API_BASE } from './config'

const TOKEN_KEY = 'token'

export function getToken() {
  return uni.getStorageSync(TOKEN_KEY) || ''
}

export function setToken(token) {
  if (token) {
    uni.setStorageSync(TOKEN_KEY, token)
  } else {
    uni.removeStorageSync(TOKEN_KEY)
  }
}

let unauthorizedHandler = null
export function onUnauthorized(fn) {
  unauthorizedHandler = fn
}

/**
 * 请求。silent=true 时不弹错误 toast（轮询等场景）
 * @returns {Promise<any>} 后端 data 字段
 */
export function request({ url, method = 'GET', data, header = {}, silent = false, timeout = 10000 }) {
  return new Promise((resolve, reject) => {
    const headerFinal = { 'Content-Type': 'application/json', ...header }
    const token = getToken()
    if (token) {
      headerFinal.Authorization = 'Bearer ' + token
    }
    uni.request({
      url: API_BASE + url,
      method,
      data,
      header: headerFinal,
      timeout,
      success: (res) => {
        const body = res.data || {}
        if (res.statusCode === 401 || body.code === 40101) {
          setToken('')
          if (unauthorizedHandler) unauthorizedHandler()
          const err = new Error(body.msg || '请先登录')
          err.code = 40101
          return reject(err)
        }
        if (res.statusCode >= 400 || (body.code !== undefined && body.code !== 0)) {
          const msg = body.msg || '请求失败，请稍后再试'
          if (!silent) {
            uni.showToast({ title: msg, icon: 'none' })
          }
          const err = new Error(msg)
          err.code = body.code
          return reject(err)
        }
        resolve(body.data !== undefined ? body.data : body)
      },
      fail: () => {
        if (!silent) {
          uni.showToast({ title: '网络异常，请稍后再试', icon: 'none' })
        }
        reject(new Error('network'))
      }
    })
  })
}

/** 上传图片到 /api/v1/uploads，返回 {url} */
export function uploadImage(filePath) {
  return new Promise((resolve, reject) => {
    const token = getToken()
    uni.uploadFile({
      url: API_BASE + '/api/v1/uploads',
      filePath,
      name: 'file',
      header: token ? { Authorization: 'Bearer ' + token } : {},
      success: (res) => {
        try {
          const body = JSON.parse(res.data)
          if (body.code === 0) {
            resolve(body.data)
          } else {
            uni.showToast({ title: body.msg || '上传失败', icon: 'none' })
            reject(new Error(body.msg))
          }
        } catch (e) {
          uni.showToast({ title: '上传失败', icon: 'none' })
          reject(e)
        }
      },
      fail: () => {
        uni.showToast({ title: '网络异常，请稍后再试', icon: 'none' })
        reject(new Error('network'))
      }
    })
  })
}
