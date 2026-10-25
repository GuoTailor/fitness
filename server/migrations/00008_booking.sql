-- 00008: P1 预约体验
-- 教练请假（当日时段屏蔽）/ 团课候补（满员排队，退出自动递补扣次）

CREATE TABLE IF NOT EXISTS coach_leave (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  owner_id BIGINT NOT NULL DEFAULT 1,
  coach_id BIGINT NOT NULL,
  leave_date DATE NOT NULL,
  start_minute INT NOT NULL DEFAULT 0,
  end_minute INT NOT NULL DEFAULT 1440,
  reason VARCHAR(200) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_leave_coach (coach_id, leave_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS group_waitlist (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  owner_id BIGINT NOT NULL DEFAULT 1,
  group_class_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  member_package_id BIGINT NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'waiting',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  promoted_at DATETIME NULL,
  UNIQUE KEY uk_wl (group_class_id, user_id),
  KEY idx_wl_class (group_class_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
