-- 00003_member_snapshot.sql
-- member_package 增加购买时快照列：开卡有效期天数、单课时折算单价（分）。
-- 退款折算与开卡计算都以快照为准，避免后台改价/删规格影响已售课包。

ALTER TABLE member_package
  ADD COLUMN valid_days INT NOT NULL DEFAULT 365 COMMENT '购买时快照：自开卡起有效天数',
  ADD COLUMN unit_price BIGINT NOT NULL DEFAULT 0 COMMENT '购买时快照：单课时折算价（分）= sku.price/sku.sessions 向下取整';
