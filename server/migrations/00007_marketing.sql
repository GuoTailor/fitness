-- 00007: P1 营销收尾
-- 限时活动价（SKU 三列 + orders.promo_used）/ 赠课券（orders/member_package.gift_sessions）/ 奖励券配置种子

ALTER TABLE package_sku
  ADD COLUMN promo_price BIGINT NULL COMMENT '限时活动价（分），NULL=未启用',
  ADD COLUMN promo_start DATETIME NULL COMMENT '活动开始时间',
  ADD COLUMN promo_end DATETIME NULL COMMENT '活动结束时间';

ALTER TABLE orders
  ADD COLUMN promo_used TINYINT NOT NULL DEFAULT 0 COMMENT '本单使用了限时活动价',
  ADD COLUMN gift_sessions INT NOT NULL DEFAULT 0 COMMENT '赠课券赠送课时数（入账快照）';

ALTER TABLE member_package
  ADD COLUMN gift_sessions INT NOT NULL DEFAULT 0 COMMENT '赠课课时（退款折算时不计入可退）';

INSERT INTO app_config (cfg_key, cfg_value, remark) VALUES
  ('marketing.review_reward_template', '', '评价奖励券模板ID，空=不发；评价审核通过后发放，每人限领由券模板控制'),
  ('marketing.renewal_reminder_template', '', '续费提醒券模板ID，空=不发；课时不足预警时自动发放，每人限领由券模板控制');
