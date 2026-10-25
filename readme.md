# 健身课包售卖微信小程序

自营健身工作室的「卖课包 + 约教练」小程序：课包商城 + 微信支付 → 预约教练/团课报名 → 线下扫码核销 → 课时跟踪 → 课后评价的完整闭环。

- 产品规划：[docs/plans/健身课包售卖微信小程序 · 产品功能规划.md](docs/plans/健身课包售卖微信小程序%20·%20产品功能规划.md)
- 实施进度：[docs/plans/进度记录.md](docs/plans/进度记录.md)
- 成本估算：[docs/plans/基础设施成本估算（服务器域名认证等）.md](docs/plans/基础设施成本估算（服务器域名认证等）.md)

## 仓库结构

| 目录 | 说明 | 技术栈 |
|---|---|---|
| `server/` | 后端 API（学员端 + 教练端 + 管理后台） | Go 1.21 · Gin · GORM · MySQL 5.7 |
| `miniprogram/` | 微信小程序（学员 + 教练双角色） | 原生小程序 |
| `admin-web/` | Web 管理后台 | Vue3 · Vite · Element Plus |

## 快速开始

### 1. 后端

```bash
cd server
# 配置见 configs/config.yaml；数据库密码放 configs/config.local.yaml（已 gitignore）
go run ./cmd/server            # 启动（自动跑迁移），默认 :8080
go run ./cmd/server -migrate-only   # 仅迁移
go test ./...                  # 单测（sqlite 内存库，不需要 MySQL）
```

- 数据库：开发用 `aos-mysql.dev.svc.cluster.local`（10.96.158.189:3306），库 `fitness`，账号 `fitness`（密码在 `.db-dev-password.txt`，仅本地）
- 默认管理员：`admin / fitness123`（首次启动自动创建，可用环境变量 `ADMIN_INIT_PASSWORD` 指定）
- **微信能力全部有 mock 开关**（`configs/config.yaml`）：`wechat.mock=true` 时登录 code 即身份（`mock-user-1` 造学员、`mock-coach-1` 造教练）、手机号 code 即手机号明文；`pay.mock=true` 时下单直接走模拟支付成功。资质办结后关 mock 填正式参数即接真。

### 2. 管理后台

```bash
cd admin-web
npm install
npm run dev    # http://localhost:5173，已代理 /admin/api 到 :8080
```

### 3. 小程序

微信开发者工具导入 `miniprogram/` 目录（`project.config.json` 已就绪，AppID 可先用测试号）。
`app.js` 里 `apiBase` 默认指向 `http://127.0.0.1:8080`；开发者工具需开启「不校验合法域名」。

## 安全要求（贯穿各里程碑）

- 登录/验证码类接口**限流防暴力破解**（admin 登录失败锁定、小程序登录频控）—— M2 落地
- 接口数据隔离：学员只能查改自己的订单/课包/预约/评价（service 层统一按 user_id 过滤，他人资源一律 404）—— 各里程碑持续
- 资金安全：支付回调验签（v3 平台证书 + APIv3 解密）+ 幂等、退款先发起微信侧再落库、课时变动唯一流水 —— 数据层已保障
- JWT 短期有效 + 敏感操作二次校验；生产强制 HTTPS（nginx 配置见 `server/deploy/`）

## 功能全景（P0 完整闭环 + P1 全量）

- **商城**：课包/SKU/教练/Banner 后台管理；下单 → 微信支付（mock 即时入账 / v3 真实通道）→ 回调 → 课时入账 → 退款（全额/剩余折算/审批）
- **营销**：优惠券三类型（满减/折扣/**赠课券**）——领券中心/后台定向/新人礼包/评价奖励/续费提醒多渠道发放；**新人价**（每人一次，手机号判重）+ **限时活动价**（倒计时）；三方比价不叠加取最优、赠课券可叠加赠课时；用券订单退款券自动返还、后台对账（领取率/使用率）
- **预约**：私教预约（周模板时段、防冲突、取消规则、**教练请假屏蔽**）+ 教练确认；团课发布/报名（防超卖）/退出；**满员候补自动递补**（退出即 FIFO 占位扣次+通知）；扫码核销（幂等）+ 后台代客操作；教练列表评分筛选
- **跟踪**：我的课时与流水、训练日历打点、周月统计、**训练周报**（环比/教练榜/体重变化）、**体重记录曲线**、**教练课后反馈**（学员可见）
- **评价与问答**：核销门槛 + 课包/教练双维度一次提交（带图 ≤4 张站内上传）、后台审核/精选、教练回复、评分聚合；**课程问答区**（先发后审、教练认证标识优先、置顶 FAQ、匿名提问）
- **合规**：隐私/用户协议 + 微信客服、个人资料编辑（头像昵称）、admin 列表手机号脱敏（教练端业务保留明文）、**操作审计日志**（12 类资金/敏感操作留痕，老板可查）
- **消息**：订阅消息（预约/报名确认、评价邀请、课时与有效期预警、候补递补、课后反馈、问答提醒）——`notify.mock` 开关 + `notify_log` 防重发 + 进程内定时任务
- **看板**：GMV（净额）、课消率、复购率、30 天趋势、教练排行、待办直跳（ECharts）
- **图片上传**：`POST /api/v1/uploads` 白名单+魔数校验、本地磁盘 + nginx `/uploads/` 静态直出

## 部署（M6）

- 一键部署：`./scripts/deploy.sh user@host`（交叉编译 linux/amd64 + admin-web 构建 + systemd 滚动重启）
- nginx / systemd / 生产配置样例 / 数据库备份：`server/deploy/`、`scripts/backup_db.sh`
- 上线步骤（备案 → 认证 → 商户号 → 订阅模板 → 配置 → 验证 → 提审）：[docs/plans/上线checklist.md](docs/plans/上线checklist.md)
- **微信支付 v3 真实通道已就绪**（`internal/wechat/pay_v3.go`，wechatpay-go）：`pay.mock=false` + 商户配置即启用；
  回调 `/api/v1/pay/notify` 验签解密后走幂等入账链路；退款在审批时先发起微信侧。真实联调待资质办结。

## 关键机制（数据层保障）

- **私教时段防冲突**：`booking.slot_token` 唯一键（占用=`coach:date:start`，取消/过期置 `done:{id}` 释放）——MySQL 5.7 无部分索引的替代方案
- **扣次幂等**：所有课时变动写 `package_transaction`，`idempotency_key` 唯一约束防重复核销/回调重放
- **团课防超卖**：`UPDATE group_class SET enrolled_count=enrolled_count+1 WHERE enrolled_count<capacity` 条件更新
- **支付回调幂等**：`payment.transaction_id` 唯一（NULL 默认值）+ 条件 UPDATE 影响行数闸门
- **平台化预留**：核心表带 `owner_id`，单店阶段恒为 1

---

## 原始需求备忘

帮我策划一个卖健身课程的微信小程序，必须有的内容是：教练介绍，课程跟踪、预约教练线下开课，课程介绍。
可以参考keep这个软件，但是主要的还是卖课程，用户可以评价留言
帮我规划app功能

多个教练/场馆入驻，收取佣金
还有周期制训练营（如21天减脂营，含课程+打卡督学+教练答疑），客单价高，但需要社群运营和教练服务投入
第二版（强化跟踪与复购） ：课后训练记录、课程跟踪仪表盘、教练回复评价、课程问答区、候补与提醒优化、教练绩效数据。
加入邀请返现
注重安全，注意接口数据安全，避免接口暴力破解
