-- 00012: P2 批次① 老带新裂变
-- 邀请关系绑定 + 首单实付后双方发券奖励（合规：只发券，不做现金佣金）

CREATE TABLE IF NOT EXISTS referral_record (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  inviter_id BIGINT NOT NULL DEFAULT 0,
  invitee_id BIGINT NOT NULL DEFAULT 0,
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1=已绑定 2=已奖励',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  rewarded_at DATETIME NULL,
  UNIQUE KEY uk_ref_invitee (invitee_id),
  KEY idx_ref_inviter (inviter_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 奖励券模板 ID（空=不发奖励，绑定不受影响）
INSERT INTO app_config (cfg_key, cfg_value, remark) VALUES
  ('marketing.referral_inviter_template', '', '老带新-邀请人奖励券模板ID，空=不发'),
  ('marketing.referral_invitee_template', '', '老带新-被邀请人奖励券模板ID，空=不发');
