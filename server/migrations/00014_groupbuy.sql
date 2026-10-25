-- 00014: P2 批次③ 拼团
-- SKU 拼团价 + 开团/参团 + 到期未成团自动原路退款

ALTER TABLE package_sku ADD COLUMN group_price BIGINT NOT NULL DEFAULT 0 COMMENT '拼团价（分），0=未开启';
ALTER TABLE package_sku ADD COLUMN group_capacity INT NOT NULL DEFAULT 0 COMMENT '成团人数，2~5，0=未开启';

ALTER TABLE orders ADD COLUMN group_buy_id BIGINT NOT NULL DEFAULT 0 COMMENT '所属拼团，0=普通单';

CREATE TABLE IF NOT EXISTS group_buy (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  owner_id BIGINT NOT NULL DEFAULT 1,
  sku_id BIGINT NOT NULL DEFAULT 0,
  package_id BIGINT NOT NULL DEFAULT 0,
  capacity INT NOT NULL DEFAULT 2,
  price BIGINT NOT NULL DEFAULT 0 COMMENT '拼团价快照（分）',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1=报名中 2=已成团 3=未成团(已退款)',
  leader_user_id BIGINT NOT NULL DEFAULT 0,
  expire_at DATETIME NOT NULL,
  success_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_gb_sku_status (sku_id, status),
  KEY idx_gb_expire (expire_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS group_buy_member (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  group_buy_id BIGINT NOT NULL DEFAULT 0,
  user_id BIGINT NOT NULL DEFAULT 0,
  order_id BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_gbm_group_user (group_buy_id, user_id),
  KEY idx_gbm_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO app_config (cfg_key, cfg_value, remark) VALUES
  ('marketing.group_expire_hours', '48', '拼团有效期（小时），到期未成团自动退款');
