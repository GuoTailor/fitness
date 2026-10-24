-- 00004: 评价自动过审开关（默认关闭：评价需后台审核后对外可见）
INSERT INTO app_config (cfg_key, cfg_value, remark) VALUES ('review.auto_approve', '0', '评价自动过审 1/0')
  ON DUPLICATE KEY UPDATE cfg_key = cfg_key;
