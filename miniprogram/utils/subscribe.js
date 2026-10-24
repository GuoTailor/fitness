// 订阅消息授权：先拉取后台已配置的模板（mock/未配置 → 空表），
// 有对应场景模板时才弹授权，避免开发环境打扰。
const { request } = require('./request')

function requestSubscribe(scenes) {
  return request({ url: '/api/v1/notify/templates' })
    .then((d) => {
      if (d.mock || !d.templates) return
      const ids = []
      scenes.forEach((s) => {
        if (d.templates[s]) ids.push(d.templates[s])
      })
      if (!ids.length) return
      wx.requestSubscribeMessage({ tmplIds: ids.slice(0, 3) })
    })
    .catch(() => {}) // 授权失败不阻断业务
}

module.exports = { requestSubscribe }
