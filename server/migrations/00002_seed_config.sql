-- 00002_seed_config.sql 运营配置初始值（取消/退款/预警规则，均后台可改）

INSERT INTO app_config (cfg_key, cfg_value, remark) VALUES
  ('booking.cancel_penalty_hours', '4', '私教开课前 N 小时内取消扣 1 课时，此前免费'),
  ('group_class.quit_deadline_hours', '4', '团课开课前 N 小时内不可退出返还课时'),
  ('refund.full_days', '7', '未开卡 N 天内全额退'),
  ('member.warn_sessions', '3', '剩余课时低于 N 节提醒'),
  ('member.warn_expire_days', '14', '距有效期不足 N 天提醒'),
  ('review.invite_after_days', '1', '核销完成 N 天后推送评价邀请')
ON DUPLICATE KEY UPDATE cfg_key = cfg_key;
