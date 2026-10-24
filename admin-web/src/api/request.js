import axios from 'axios'
import { ElMessage } from 'element-plus'

// 统一响应 {code, msg, data}；401 跳登录页
const http = axios.create({ baseURL: '/', timeout: 15000 })

http.interceptors.request.use((config) => {
  const token = localStorage.getItem('admin_token')
  if (token) config.headers.Authorization = 'Bearer ' + token
  return config
})

http.interceptors.response.use(
  (resp) => {
    const body = resp.data
    if (body.code === 0) return body.data
    ElMessage.error(body.msg || '请求失败')
    return Promise.reject(new Error(body.msg))
  },
  (err) => {
    const body = err.response?.data
    if (err.response?.status === 401 || body?.code === 40101) {
      localStorage.removeItem('admin_token')
      if (!location.hash.includes('#/login')) location.hash = '#/login'
    }
    ElMessage.error(body?.msg || '网络异常')
    return Promise.reject(err)
  }
)

export default http
