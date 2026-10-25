import http from './request'

// ---- 课包 ----
export const listPackages = (params) => http.get('/admin/api/v1/packages', { params })
export const savePackage = (data) =>
  data.id > 0 ? http.post(`/admin/api/v1/packages/${data.id}`, data) : http.post('/admin/api/v1/packages', data)

// ---- 教练 ----
export const listCoaches = (params) => http.get('/admin/api/v1/coaches', { params })
export const saveCoach = (data) =>
  data.id > 0 ? http.post(`/admin/api/v1/coaches/${data.id}`, data) : http.post('/admin/api/v1/coaches', data)
export const verifyCoachCert = (id, verified) => http.post(`/admin/api/v1/coaches/${id}/verify`, { verified })

// ---- Banner ----
export const listBanners = () => http.get('/admin/api/v1/banners')
export const saveBanner = (data) =>
  data.id > 0 ? http.post(`/admin/api/v1/banners/${data.id}`, data) : http.post('/admin/api/v1/banners', data)
export const deleteBanner = (id) => http.delete(`/admin/api/v1/banners/${id}`)

// ---- 订单 / 退款 ----
export const listOrders = (params) => http.get('/admin/api/v1/orders', { params })
export const getOrder = (id) => http.get(`/admin/api/v1/orders/${id}`)
export const listRefunds = (params) => http.get('/admin/api/v1/refunds', { params })
export const approveRefund = (id) => http.post(`/admin/api/v1/refunds/${id}/approve`)
export const rejectRefund = (id, reason) => http.post(`/admin/api/v1/refunds/${id}/reject`, { reason })

// ---- 学员 ----
export const listMembers = (params) => http.get('/admin/api/v1/members', { params })
export const memberPackages = (id) => http.get(`/admin/api/v1/members/${id}/packages`)
export const adjustSessions = (uid, mpId, data) =>
  http.post(`/admin/api/v1/members/${uid}/packages/${mpId}/adjust`, data)

// ---- 配置 ----
export const listConfigs = () => http.get('/admin/api/v1/configs')
export const saveConfig = (data) => http.post('/admin/api/v1/configs', data)

// ---- 评价管理 ----
export const listReviews = (params) => http.get('/admin/api/v1/reviews', { params })
export const auditReview = (id, action) => http.post(`/admin/api/v1/reviews/${id}/audit`, { action })
export const featureReview = (id, featured) => http.post(`/admin/api/v1/reviews/${id}/featured`, { featured })

// ---- 优惠券 ----
export const listCoupons = () => http.get('/admin/api/v1/coupons')

// 问答区管理
export const listQa = (params) => http.get('/admin/api/v1/qa', { params })
export const setQaStatus = (id, status) => http.post(`/admin/api/v1/qa/questions/${id}/status`, { status })
export const setQaAnswerStatus = (id, status) => http.post(`/admin/api/v1/qa/answers/${id}/status`, { status })
export const setQaPin = (id, pinned) => http.post(`/admin/api/v1/qa/questions/${id}/pin`, { pinned })

// 操作审计日志
export const listAuditLogs = (params) => http.get('/admin/api/v1/audit-logs', { params })
export const saveCoupon = (data) =>
  data.id > 0 ? http.post(`/admin/api/v1/coupons/${data.id}`, data) : http.post('/admin/api/v1/coupons', data)
export const toggleCoupon = (id, status) => http.post(`/admin/api/v1/coupons/${id}/status`, { status })
export const grantCoupon = (id, userId) => http.post(`/admin/api/v1/coupons/${id}/grant`, { userId })

// 金额（分）→ 元
export const yuan = (cents) => (cents == null ? '—' : (cents / 100).toFixed(2))
