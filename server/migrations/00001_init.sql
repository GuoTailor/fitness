-- 00001_init.sql
-- 健身课包售卖小程序 P0 全量建表（兼容 MySQL 5.7：不使用 CHECK/函数索引/默认表达式）
-- 约定：金额单位「分」BIGINT；时间 DATETIME；状态用 VARCHAR 存枚举便于排查；
--      核心业务表带 owner_id 归属字段（平台化预留，单店阶段恒为 1）。

-- ============ 用户与角色 ============

CREATE TABLE user (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  openid VARCHAR(64) NOT NULL DEFAULT '',
  unionid VARCHAR(64) NOT NULL DEFAULT '',
  nickname VARCHAR(64) NOT NULL DEFAULT '',
  avatar_url VARCHAR(512) NOT NULL DEFAULT '',
  phone VARCHAR(20) NOT NULL DEFAULT '',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1正常 0禁用',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_user_openid (openid),
  KEY idx_user_phone (phone)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='学员';

CREATE TABLE coach (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  owner_id BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '归属（平台化预留）',
  user_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '绑定登录账号，0=未绑定',
  name VARCHAR(64) NOT NULL DEFAULT '',
  avatar_url VARCHAR(512) NOT NULL DEFAULT '',
  gender TINYINT NOT NULL DEFAULT 0 COMMENT '0未知 1男 2女',
  years INT NOT NULL DEFAULT 0 COMMENT '从业年限',
  specialties VARCHAR(255) NOT NULL DEFAULT '' COMMENT '擅长领域，逗号分隔',
  cert_images TEXT COMMENT '资质证书图片，JSON 数组',
  cert_verified TINYINT NOT NULL DEFAULT 0 COMMENT '资质审核 0未审 1通过',
  intro TEXT COMMENT '自我介绍（富文本）',
  intro_video_url VARCHAR(512) NOT NULL DEFAULT '',
  rating_avg DECIMAL(3,2) NOT NULL DEFAULT 0.00 COMMENT '评分聚合',
  rating_count INT NOT NULL DEFAULT 0,
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1在职 0停用',
  sort INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_coach_owner (owner_id, status),
  KEY idx_coach_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='教练';

CREATE TABLE admin_user (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  username VARCHAR(64) NOT NULL,
  password_hash VARCHAR(100) NOT NULL,
  name VARCHAR(64) NOT NULL DEFAULT '',
  role VARCHAR(16) NOT NULL DEFAULT 'receptionist' COMMENT 'boss老板 / receptionist前台',
  status TINYINT NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_admin_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='后台账号';

-- ============ 商城 ============

CREATE TABLE course_package (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  owner_id BIGINT UNSIGNED NOT NULL DEFAULT 1,
  title VARCHAR(128) NOT NULL DEFAULT '',
  cover_url VARCHAR(512) NOT NULL DEFAULT '',
  intro_video_url VARCHAR(512) NOT NULL DEFAULT '' COMMENT '介绍视频（自建 nginx 静态目录）',
  category VARCHAR(16) NOT NULL DEFAULT 'private' COMMENT 'private私教/group团课/trial体验',
  tags VARCHAR(255) NOT NULL DEFAULT '' COMMENT '训练目标，逗号分隔：减脂,增肌,塑形,康复',
  difficulty TINYINT NOT NULL DEFAULT 1 COMMENT '难度 1-5',
  outline TEXT COMMENT '课程大纲（教什么）',
  suitable TEXT COMMENT '适合人群',
  status TINYINT NOT NULL DEFAULT 0 COMMENT '1上架 0下架',
  sold_count INT NOT NULL DEFAULT 0,
  sort INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_pkg_owner (owner_id, status, category)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='课包';

CREATE TABLE package_sku (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  package_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '如：10次卡',
  sessions INT NOT NULL DEFAULT 0 COMMENT '课时数',
  valid_days INT NOT NULL DEFAULT 365 COMMENT '自开卡起有效天数',
  price BIGINT NOT NULL DEFAULT 0 COMMENT '售价（分）',
  original_price BIGINT NOT NULL DEFAULT 0 COMMENT '原价（分）',
  newcomer_price BIGINT COMMENT '新人价（分），P1 预留',
  status TINYINT NOT NULL DEFAULT 1,
  sort INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_sku_pkg (package_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='课包规格';

CREATE TABLE coach_package (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  coach_id BIGINT UNSIGNED NOT NULL,
  package_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_coach_pkg (coach_id, package_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='教练在售课包';

CREATE TABLE banner (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  owner_id BIGINT UNSIGNED NOT NULL DEFAULT 1,
  image_url VARCHAR(512) NOT NULL DEFAULT '',
  link_type VARCHAR(16) NOT NULL DEFAULT 'none' COMMENT 'none/package/coach',
  link_value VARCHAR(128) NOT NULL DEFAULT '',
  sort INT NOT NULL DEFAULT 0,
  status TINYINT NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_banner_owner (owner_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='首页运营位';

-- ============ 交易 ============

CREATE TABLE orders (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  order_no VARCHAR(32) NOT NULL,
  owner_id BIGINT UNSIGNED NOT NULL DEFAULT 1,
  user_id BIGINT UNSIGNED NOT NULL,
  package_id BIGINT UNSIGNED NOT NULL,
  sku_id BIGINT UNSIGNED NOT NULL,
  title VARCHAR(128) NOT NULL DEFAULT '' COMMENT '下单快照',
  amount BIGINT NOT NULL DEFAULT 0 COMMENT '实付（分）',
  coupon_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '优惠券 P1 预留',
  coupon_amount BIGINT NOT NULL DEFAULT 0 COMMENT '券抵扣（分）P1 预留',
  status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/paid/closed/refunding/refunded',
  pay_time DATETIME COMMENT '支付完成时间',
  transaction_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT '微信支付单号',
  refund_amount BIGINT NOT NULL DEFAULT 0 COMMENT '累计退款（分）',
  remark VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_order_no (order_no),
  KEY idx_order_user (user_id, status),
  KEY idx_order_pkg (package_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='订单';

CREATE TABLE payment (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  order_id BIGINT UNSIGNED NOT NULL,
  transaction_id VARCHAR(64) NULL DEFAULT NULL COMMENT '微信支付单号：回调写入，唯一键 NULL 默认值允许多行待支付（防回调重放的 DB 级兜底）',
  amount BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL DEFAULT 'created' COMMENT 'created/success/failed',
  raw_notify TEXT COMMENT '回调原文备查',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_payment_order (order_id),
  UNIQUE KEY uk_payment_txn (transaction_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='支付流水';

CREATE TABLE refund (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  refund_no VARCHAR(32) NOT NULL,
  order_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  amount BIGINT NOT NULL DEFAULT 0 COMMENT '退款金额（分）',
  reason VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/approved/rejected/refunded/failed',
  reject_reason VARCHAR(255) NOT NULL DEFAULT '',
  admin_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_refund_no (refund_no),
  KEY idx_refund_order (order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='退款单';

-- ============ 课时账户 ============

CREATE TABLE member_package (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  owner_id BIGINT UNSIGNED NOT NULL DEFAULT 1,
  user_id BIGINT UNSIGNED NOT NULL,
  package_id BIGINT UNSIGNED NOT NULL,
  sku_id BIGINT UNSIGNED NOT NULL,
  order_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0=后台手动调整等非购买来源',
  package_title VARCHAR(128) NOT NULL DEFAULT '' COMMENT '快照',
  total_sessions INT NOT NULL DEFAULT 0,
  used_sessions INT NOT NULL DEFAULT 0 COMMENT '冗余余额=流水累计，以流水为准对账',
  expire_date DATE COMMENT '开卡时 = 激活日 + valid_days',
  status VARCHAR(16) NOT NULL DEFAULT 'inactive' COMMENT 'inactive未开卡/active/exhausted用完/expired过期/refunded',
  activated_at DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_mp_user (user_id, status),
  KEY idx_mp_expire (status, expire_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='学员课包（课时账户）';

CREATE TABLE package_transaction (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  member_package_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  sessions_change INT NOT NULL DEFAULT 0 COMMENT '正入账 负扣减',
  type VARCHAR(24) NOT NULL COMMENT 'purchase购入/deduct核销扣除/booking_cancel返还/penalty取消扣次/refund_reclaim退款回收/manual_adjust手动调整',
  idempotency_key VARCHAR(64) NOT NULL COMMENT '业务幂等键（如 verify:{verificationID}）',
  biz_type VARCHAR(16) NOT NULL DEFAULT '',
  biz_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  remark VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_txn_idem (idempotency_key),
  KEY idx_txn_mp (member_package_id),
  KEY idx_txn_user (user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='课时流水（所有课时变动唯一入口）';

-- ============ 预约与团课 ============

CREATE TABLE coach_schedule (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  coach_id BIGINT UNSIGNED NOT NULL,
  weekday TINYINT NOT NULL COMMENT '0-6 周日-周六',
  start_minute INT NOT NULL COMMENT '当日 0 点起分钟数，30 分钟粒度',
  end_minute INT NOT NULL,
  status TINYINT NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_schedule (coach_id, weekday, start_minute)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='教练每周可约时段模板';

CREATE TABLE booking (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  owner_id BIGINT UNSIGNED NOT NULL DEFAULT 1,
  user_id BIGINT UNSIGNED NOT NULL,
  coach_id BIGINT UNSIGNED NOT NULL,
  package_id BIGINT UNSIGNED NOT NULL,
  member_package_id BIGINT UNSIGNED NOT NULL,
  booking_date DATE NOT NULL,
  start_minute INT NOT NULL,
  end_minute INT NOT NULL,
  slot_token VARCHAR(64) NOT NULL DEFAULT '' COMMENT '时段占用凭证：占用时={coach_id}:{date}:{start_minute}；取消/过期/完成置为 done:{id} 释放时段（MySQL 5.7 无部分索引的替代方案）',
  status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending待确认/confirmed已确认/completed已完成/cancelled已取消/expired',
  cancel_by VARCHAR(8) NOT NULL DEFAULT '' COMMENT 'user/coach/admin',
  cancel_time DATETIME,
  penalty_sessions INT NOT NULL DEFAULT 0 COMMENT '取消扣的课时数',
  remark VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_booking_slot (slot_token) COMMENT '并发防冲突兜底，slot_token 随状态释放',
  KEY idx_booking_slot_day (coach_id, booking_date, status),
  KEY idx_booking_user (user_id, status),
  KEY idx_booking_date (booking_date, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='私教预约';

CREATE TABLE group_class (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  owner_id BIGINT UNSIGNED NOT NULL DEFAULT 1,
  coach_id BIGINT UNSIGNED NOT NULL,
  package_id BIGINT UNSIGNED NOT NULL,
  title VARCHAR(128) NOT NULL DEFAULT '',
  class_date DATE NOT NULL,
  start_minute INT NOT NULL,
  end_minute INT NOT NULL,
  location VARCHAR(255) NOT NULL DEFAULT '',
  capacity INT NOT NULL DEFAULT 0,
  enrolled_count INT NOT NULL DEFAULT 0 COMMENT '乐观锁防超卖',
  status VARCHAR(16) NOT NULL DEFAULT 'enrolling' COMMENT 'enrolling报名中/fulfilled满员/closed停止报名/cancelled/finished',
  cancel_reason VARCHAR(255) NOT NULL DEFAULT '',
  remark VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_gc_date (class_date, status),
  KEY idx_gc_coach (coach_id, class_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='团课';

CREATE TABLE group_enrollment (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  group_class_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  member_package_id BIGINT UNSIGNED NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'enrolled' COMMENT 'enrolled已报名/quit已退出/verified已核销；退出后重报=翻回本行（uk_ge 决定每人每课仅一行，不另插）',
  quit_time DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_ge (group_class_id, user_id),
  KEY idx_ge_user (user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='团课报名';

-- ============ 核销与训练 ============

CREATE TABLE verification (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  biz_type VARCHAR(16) NOT NULL COMMENT 'private/group',
  booking_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  enrollment_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  user_id BIGINT UNSIGNED NOT NULL,
  coach_id BIGINT UNSIGNED NOT NULL,
  member_package_id BIGINT UNSIGNED NOT NULL,
  sessions INT NOT NULL DEFAULT 1 COMMENT '本次扣课时数',
  code VARCHAR(32) NOT NULL COMMENT '学员出示的核销码',
  status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/verified/cancelled/expired',
  verified_at DATETIME,
  verified_by VARCHAR(16) NOT NULL DEFAULT '' COMMENT 'coach/admin',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_verify_code (code),
  KEY idx_verify_user (user_id, status),
  KEY idx_verify_coach (coach_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='核销记录';

CREATE TABLE training_record (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  coach_id BIGINT UNSIGNED NOT NULL,
  package_id BIGINT UNSIGNED NOT NULL,
  record_date DATE NOT NULL,
  project VARCHAR(64) NOT NULL DEFAULT '' COMMENT '项目（课包名/团课名）',
  duration_minutes INT NOT NULL DEFAULT 0,
  source VARCHAR(16) NOT NULL DEFAULT 'private' COMMENT 'private/group',
  remark VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_tr_user (user_id, record_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='训练记录（核销自动生成）';

-- ============ 评价 ============

CREATE TABLE review (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  owner_id BIGINT UNSIGNED NOT NULL DEFAULT 1,
  user_id BIGINT UNSIGNED NOT NULL,
  target_type VARCHAR(16) NOT NULL COMMENT 'package/coach 双维度',
  package_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  coach_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  verification_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '关联核销记录（防刷单依据）',
  rating TINYINT NOT NULL DEFAULT 5,
  content TEXT,
  images TEXT COMMENT 'JSON 数组',
  status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending待审/approved通过/hidden隐藏',
  featured TINYINT NOT NULL DEFAULT 0 COMMENT '精选',
  coach_reply TEXT,
  coach_reply_at DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_review (verification_id, target_type) COMMENT '同一核销每维度限评一次（防双击重复提交）',
  KEY idx_review_pkg (target_type, package_id, status),
  KEY idx_review_coach (target_type, coach_id, status),
  KEY idx_review_status (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='评价';

-- ============ 配置与消息 ============

CREATE TABLE app_config (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  cfg_key VARCHAR(64) NOT NULL,
  cfg_value VARCHAR(512) NOT NULL DEFAULT '',
  remark VARCHAR(255) NOT NULL DEFAULT '',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_cfg_key (cfg_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='运营配置';

CREATE TABLE notify_log (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  scene VARCHAR(32) NOT NULL COMMENT 'booking_confirmed/review_invite/sessions_warning/expire_warning',
  biz_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  status VARCHAR(8) NOT NULL DEFAULT 'sent' COMMENT 'sent/fail',
  err_msg VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_notify (user_id, scene, biz_id) COMMENT '防重发'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='订阅消息发送记录';
