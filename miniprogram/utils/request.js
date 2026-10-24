// 统一请求封装：注入 token、拆包 {code,msg,data}、401 自动清登录态
function request({ url, method = 'GET', data = {}, header = {} }) {
  const app = getApp()
  return new Promise((resolve, reject) => {
    wx.request({
      url: app.globalData.apiBase + url,
      method,
      data,
      timeout: 10000,
      header: {
        'Content-Type': 'application/json',
        ...(wx.getStorageSync('token') ? { Authorization: 'Bearer ' + wx.getStorageSync('token') } : {}),
        ...header,
      },
      success(res) {
        const body = res.data || {}
        if (res.statusCode === 401 || body.code === 40101) {
          wx.removeStorageSync('token')
          const err = new Error(body.msg || '登录已失效')
          err.code = 40101
          return reject(err)
        }
        if (body.code !== 0) {
          const err = new Error(body.msg || '请求失败')
          err.code = body.code
          wx.showToast({ title: err.message, icon: 'none' })
          return reject(err)
        }
        resolve(body.data)
      },
      fail(err) {
        wx.showToast({ title: '网络异常，请稍后再试', icon: 'none' })
        reject(err)
      },
    })
  })
}

module.exports = { request }
