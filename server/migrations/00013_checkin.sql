-- 00013: P2 批次② 打卡挑战
-- 月度上课 N 次返券：每人每月限领一次（uk 兜底），奖励走券模板

CREATE TABLE IF NOT EXISTS checkin_reward (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NOT NULL DEFAULT 0,
  period VARCHAR(7) NOT NULL DEFAULT '' COMMENT '归属月份 2026-10',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_ck_user_period (user_id, period)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO app_config (cfg_key, cfg_value, remark) VALUES
  ('marketing.checkin_target', '', '打卡挑战：每月上课达标次数，空=关闭'),
  ('marketing.checkin_reward_template', '', '打卡挑战奖励券模板 ID，空=不发');
