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

// 图片上传：multipart 字段 file，返回 {url}
function uploadFile(filePath) {
  const app = getApp()
  return new Promise((resolve, reject) => {
    wx.uploadFile({
      url: app.globalData.apiBase + '/api/v1/uploads',
      filePath,
      name: 'file',
      header: { Authorization: 'Bearer ' + (wx.getStorageSync('token') || '') },
      success(res) {
        let body = {}
        try { body = JSON.parse(res.data || '{}') } catch (e) { /* ignore */ }
        if (res.statusCode === 401 || body.code === 40101) {
          wx.removeStorageSync('token')
          return reject(new Error('登录已失效'))
        }
        if (body.code !== 0) {
          wx.showToast({ title: body.msg || '上传失败', icon: 'none' })
          return reject(new Error(body.msg || '上传失败'))
        }
        resolve(body.data.url)
      },
      fail(err) {
        wx.showToast({ title: '上传失败，请检查网络', icon: 'none' })
        reject(err)
      },
    })
  })
}

module.exports = { request, uploadFile }
