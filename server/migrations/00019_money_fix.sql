-- 00019: 拼团占席原子化 + 退款失败终态字段 + 订单拼团索引（第三轮审查 P0-1/P1-4）
-- 占席计数：含未支付成员；关单/退款终态释放。成团判定仍以 paidCount（仅 paid）为准。
ALTER TABLE group_buy ADD COLUMN joined_count INT NOT NULL DEFAULT 0 COMMENT '当前占席数(含未支付,关单/退款释放)';

-- 存量回填：占席 = 有效成员行（pending/paid/refunding 订单）
UPDATE group_buy g SET g.joined_count = (
    SELECT COUNT(*) FROM group_buy_member m JOIN orders o ON o.id = m.order_id
    WHERE m.group_buy_id = g.id AND o.status IN ('pending', 'paid', 'refunding')
);

-- 死团存量清理：closed/refunded 订单的成员行不再占座（用户可重新参团）
DELETE m FROM group_buy_member m JOIN orders o ON o.id = m.order_id WHERE o.status IN ('closed', 'refunded');

-- 退款失败终态：连续异常达阈值转 failed 人工处理；恢复课时需要审批前账户状态快照
ALTER TABLE refund ADD COLUMN abnormal_count INT NOT NULL DEFAULT 0 COMMENT '退款异常次数(达阈值转人工)';
ALTER TABLE refund ADD COLUMN prev_mp_status VARCHAR(16) NOT NULL DEFAULT '' COMMENT '审批前课时账户状态(失败恢复用)';

-- 拼团订单查询索引（paidCount / Detail / autoRefundGroup 均按 group_buy_id 查）
CREATE INDEX idx_order_groupbuy ON orders (group_buy_id);
