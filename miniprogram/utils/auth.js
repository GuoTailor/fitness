// 登录流程：wx.login 拿 code → 后端 /auth/login 换 token。
// 后端 wechat.mock=true 时：不传 code 参数则用 wx.login 的 code 当身份，
// 传 customCode 则以固定身份登录（开发联调造号用）。
const { request } = require('./request')

async function login(customCode) {
  let code = customCode
  if (!code) {
    const res = await wx.login()
    code = res.code
  }
  const data = await request({
    url: '/api/v1/auth/login',
    method: 'POST',
    data: { code },
  })
  wx.setStorageSync('token', data.token)
  const app = getApp()
  app.globalData.user = data.user
  app.globalData.isCoach = data.isCoach
  app.globalData.coachId = data.coachId || 0
  return data
}

module.exports = { login }
