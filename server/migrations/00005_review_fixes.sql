-- 00005: 第二轮代码审查修复
-- P0-3 退款审批快照 / P0-4 团课报名世代 / M5 订单 SKU 快照 / M9 存量回填

ALTER TABLE refund
  ADD COLUMN unit_price BIGINT NOT NULL DEFAULT 0 COMMENT '审批时快照：单课时折算价(分)',
  ADD COLUMN reclaimed_sessions INT NOT NULL DEFAULT 0 COMMENT '审批时快照：回收课时数',
  ADD COLUMN calc_type VARCHAR(16) NOT NULL DEFAULT '' COMMENT 'full=全额/remainder=剩余折算/full_no_mp=无课时账户全额';

ALTER TABLE group_enrollment
  ADD COLUMN enroll_seq INT NOT NULL DEFAULT 1 COMMENT '报名世代：quit→enrolled 复用行时 +1，扣次/返还幂等键带世代防撞';

ALTER TABLE orders
  ADD COLUMN sku_sessions INT NOT NULL DEFAULT 0 COMMENT '下单快照：课时数（SKU 被删/改价后入账仍以本单为准）',
  ADD COLUMN sku_valid_days INT NOT NULL DEFAULT 0 COMMENT '下单快照：有效期天数',
  ADD COLUMN sku_unit_price BIGINT NOT NULL DEFAULT 0 COMMENT '下单快照：单课时折算价(分)';

-- 存量回填（M9）：M3 之前建的老账户 unit_price=0 → 按 SKU 回填；M5 之前的未终态订单补快照
UPDATE member_package mp JOIN package_sku s ON mp.sku_id = s.id
  SET mp.unit_price = IF(s.sessions > 0, FLOOR(s.price / s.sessions), 0),
      mp.valid_days = s.valid_days
  WHERE mp.unit_price = 0 AND s.sessions > 0;

UPDATE orders o JOIN package_sku s ON o.sku_id = s.id
  SET o.sku_sessions = s.sessions,
      o.sku_valid_days = s.valid_days,
      o.sku_unit_price = IF(s.sessions > 0, FLOOR(s.price / s.sessions), 0)
  WHERE o.sku_sessions = 0 AND s.sessions > 0
    AND o.status IN ('pending', 'paid', 'refunding');
