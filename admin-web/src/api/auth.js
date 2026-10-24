import http from './request'

export function login(data) {
  return http.post('/admin/api/v1/auth/login', data)
}
