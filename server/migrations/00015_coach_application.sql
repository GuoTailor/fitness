-- 00015: P2 批次④ 教练入驻
-- 小程序端申请 → 后台审核 → 通过自动建档并绑定账号（资质图复用站内上传）

CREATE TABLE IF NOT EXISTS coach_application (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  owner_id BIGINT NOT NULL DEFAULT 1,
  user_id BIGINT NOT NULL DEFAULT 0,
  name VARCHAR(64) NOT NULL DEFAULT '',
  gender TINYINT NOT NULL DEFAULT 0 COMMENT '0未知 1男 2女',
  years INT NOT NULL DEFAULT 0,
  specialties VARCHAR(255) NOT NULL DEFAULT '',
  cert_images TEXT COMMENT '资质证书图片 JSON 数组（站内上传 URL）',
  intro TEXT,
  phone VARCHAR(20) NOT NULL DEFAULT '' COMMENT '联系方式（便于沟通，未绑手机号也可填）',
  status TINYINT NOT NULL DEFAULT 0 COMMENT '0=待审核 1=已通过 2=已驳回',
  reject_reason VARCHAR(255) NOT NULL DEFAULT '',
  admin_id BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_ca_user (user_id, status),
  KEY idx_ca_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
