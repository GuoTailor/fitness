-- 00018: 网页端短信验证码登录（H5 无法用 wx.login，改用手机号+验证码）
CREATE TABLE IF NOT EXISTS sms_verification (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    phone VARCHAR(20) NOT NULL COMMENT '手机号',
    code VARCHAR(8) NOT NULL COMMENT '验证码',
    purpose VARCHAR(20) NOT NULL DEFAULT 'login' COMMENT '用途',
    attempts INT NOT NULL DEFAULT 0 COMMENT '错误尝试次数(达上限作废)',
    expires_at DATETIME NOT NULL COMMENT '失效时间',
    used_at DATETIME NULL COMMENT '核销时间(防重放)',
    created_at DATETIME,
    KEY idx_smsv_phone (phone, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='短信验证码';
