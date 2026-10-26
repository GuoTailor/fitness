# 上线 Checklist

> 按序勾选；全部完成后小程序提交审核。域名以下用 `your-domain.com` 代称。

## A. 资质与合规（周期最长，最先启动）

- [ ] 域名购买 + 实名（国内注册商）
- [ ] ICP 备案（小程序服务器必须在国内 + 已备案域名；约 1~3 周）
- [ ] 微信小程序认证（企业主体，¥300/年）——个人主体无法开通微信支付
- [ ] 「用户隐私保护指引」在小程序后台填写并生效（收集：手机号、昵称头像、训练记录）
- [ ] 隐私协议/服务条款页面（商城页脚可跳转）
- [ ] 微信支付商户号申请（企业资质 + 对公账户）；类目选「体育服务-体育培训」
- [ ] 商户号关联小程序 AppID；开通 JSAPI 支付与退款权限

## B. 微信后台配置

- [ ] 小程序「服务器域名」：request 合法域名 = `https://your-domain.com`（upload/download 同理）
- [ ] AppSecret 生成并配置到后端 `wechat.secret`
- [ ] 订阅消息模板申请（5 个场景：预约确认/新预约/评价邀请/课时预警/到期预警），模板 ID 填 `notify.templates`
- [ ] 支付v3：商户 API 证书下载（apiclient_key.pem 上传服务器 `/opt/fitness/secrets/`，权限 600）、
      证书序列号、APIv3 密钥配置；回调地址 = `https://your-domain.com/api/v1/pay/notify`

## C. 服务器与部署

- [ ] 服务器（2C4G 起）+ MySQL 5.7+（内网/本机，仅监听 127.0.0.1 或内网）
- [ ] 生产库建库建账号（`server/migrations` 首次启动自动建表；账号仅授权 fitness 库）
- [ ] nginx 配置（`server/deploy/nginx.conf`）+ certbot 签 SSL
- [ ] systemd 服务（`server/deploy/fitness-server.service`）+ EnvironmentFile（DB_PASSWORD/JWT_SECRET）
- [ ] 生产配置（`server/deploy/config.production.yaml.sample` 复制改）：
      `wechat.mock=false`、`pay.mock=false`、`notify.mock=false`、`mode=release`
- [ ] 首次启动：`ADMIN_INIT_PASSWORD=<强密码> systemctl start fitness-server`（改默认管理员密码）
- [ ] 备份 cron：`scripts/backup_db.sh`（每日 3:30，保留 14 天，异机同步）
- [ ] 课程视频：后台「课包管理 → 介绍视频」直接上传（落 `video.dir`，默认 `/var/www/videos`，需对 `fitness` 用户可写）；也可 scp 到 `/var/www/videos/` 后手填 URL

## D. 上线前验证（真资质后）

- [ ] 真实 wx.login → 后端 code2session（关 wechat.mock）
- [ ] 真实支付：0.01 元体验课全链路（下单→支付→回调→课时到账→退款）
- [ ] 订阅消息真实触达（授权弹窗→预约→模板消息到达）
- [ ] 小程序「体验版」全流程走查：商城/预约/团课/核销/评价/我的课时
- [ ] admin-web 生产可访问、修改 `ADMIN_INIT_PASSWORD` 后默认口令失效
- [ ] 错误路径：余额不足下单、重复核销、并发预约冲突提示正常
- [ ] 压测（可选）：/api/v1/home、packages 至少 50 并发无 5xx

## E. 提审与发布

- [ ] 类目：体育服务-体育培训（与商户号一致）
- [ ] 提审前 `pay.mock` 已关（审核期间支付真实可用）
- [ ] 版本备注附测试账号（体验学员 + 已购课包说明）
- [ ] 审核通过 → 全量发布 → 线上回归一遍 D 项

## F. 运维备忘

- 看板：`/admin/` 数据看板（GMV/课消率/复购率/教练排行）
- 日志：`journalctl -u fitness-server -f`；订阅消息发送记录在 `notify_log` 表
- 排障速查：课时不平 → 查 `package_transaction`（流水为准）；订单状态异常 → 查 `payment`/回调日志
