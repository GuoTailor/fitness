-- 00016: P2 批次⑤ 工程收尾 · 索引漂移修复
-- verification.booking_id 核销单按预约回查（模型侧有索引声明、SQL 侧漏建）

ALTER TABLE verification ADD INDEX idx_verify_booking (booking_id);
