-- 00006: P1 营销一期底座
-- 优惠券两张表 / orders.newcomer_used / app_config.cfg_value 扩 TEXT / user.phone 唯一索引 / 新配置种子

CREATE TABLE IF NOT EXISTS coupon_template (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  owner_id BIGINT NOT NULL DEFAULT 1,
  name VARCHAR(64) NOT NULL DEFAULT '',
  type VARCHAR(16) NOT NULL DEFAULT 'reduce',
  threshold BIGINT NOT NULL DEFAULT 0,
  amount BIGINT NOT NULL DEFAULT 0,
  package_id BIGINT NOT NULL DEFAULT 0,
  total_count INT NOT NULL DEFAULT 0,
  issued_count INT NOT NULL DEFAULT 0,
  per_user_limit INT NOT NULL DEFAULT 1,
  valid_days INT NOT NULL DEFAULT 30,
  status TINYINT NOT NULL DEFAULT 1,
  remark VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_coupon (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NOT NULL DEFAULT 0,
  template_id BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL DEFAULT 'usable',
  order_id BIGINT NOT NULL DEFAULT 0,
  expire_at DATETIME NULL,
  used_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_uc_user (user_id, status),
  KEY idx_uc_tpl (template_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE orders
  ADD COLUMN newcomer_used TINYINT NOT NULL DEFAULT 0 COMMENT '本单使用了新人价（每人限购一次，按手机号判重）';

ALTER TABLE app_config MODIFY cfg_value TEXT;

-- phone 唯一索引前置：列改可空 + 空串清成 NULL（'' 会撞唯一键），去掉旧普通索引
ALTER TABLE user MODIFY phone VARCHAR(20) NULL;
UPDATE user SET phone = NULL WHERE phone = '';
-- 存量重复手机号预检（P1-9）：保留最早账号的手机号，其余置 NULL，避免唯一索引报 1062 卡启动
UPDATE user SET phone = NULL
WHERE phone IS NOT NULL AND id NOT IN (
  SELECT keep_id FROM (
    SELECT MIN(id) AS keep_id FROM user WHERE phone IS NOT NULL GROUP BY phone
  ) t
);
ALTER TABLE user DROP INDEX idx_user_phone;
ALTER TABLE user ADD UNIQUE INDEX uk_user_phone (phone);

INSERT INTO app_config (cfg_key, cfg_value, remark) VALUES
  ('marketing.newcomer_gift_templates', '', '新人礼包券模板ID逗号分隔，空=不自动发放'),
  ('misc.privacy_content', '', '隐私政策正文（后台可编辑，小程序协议页展示）'),
  ('misc.agreement_content', '', '用户协议正文（后台可编辑，小程序协议页展示）');
