-- 00020: 短信发送频控计数器（第三轮审查 P1-11）
-- 单行条件自增，原子校验 60s 重发与单日上限，防并发穿透导致短信费资损。
CREATE TABLE IF NOT EXISTS sms_send_counter (
    phone VARCHAR(20) NOT NULL COMMENT '手机号',
    send_date DATE NOT NULL COMMENT '自然日',
    send_count INT NOT NULL DEFAULT 0 COMMENT '当日已发条数',
    last_sent_at DATETIME NULL COMMENT '最近发送时间(60s 重发窗口)',
    PRIMARY KEY (phone, send_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='短信发送频控计数器(原子)';
