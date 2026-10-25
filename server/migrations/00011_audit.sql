-- 00011: P1 合规收尾
-- 后台操作审计日志（资金/敏感操作留痕，老板可查）

CREATE TABLE admin_audit_log (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  admin_id BIGINT NOT NULL DEFAULT 0,
  username VARCHAR(50) NOT NULL DEFAULT '',
  action VARCHAR(50) NOT NULL DEFAULT '',
  target_type VARCHAR(30) NOT NULL DEFAULT '',
  target_id BIGINT NOT NULL DEFAULT 0,
  detail VARCHAR(1000) NOT NULL DEFAULT '',
  ip VARCHAR(45) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_al_admin (admin_id),
  KEY idx_al_time (created_at),
  KEY idx_al_action (action)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
