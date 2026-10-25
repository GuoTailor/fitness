-- 00017: 拼团活动层 —— 运营可管理的限时拼团活动（覆盖 SKU 常开拼团配置）
CREATE TABLE IF NOT EXISTS group_buy_activity (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    owner_id BIGINT NOT NULL DEFAULT 1,
    title VARCHAR(64) NOT NULL COMMENT '活动标题',
    sku_id BIGINT NOT NULL COMMENT '作用规格',
    package_id BIGINT NOT NULL,
    group_price BIGINT NOT NULL COMMENT '拼团价(分)',
    group_capacity INT NOT NULL COMMENT '成团人数',
    start_at DATETIME NOT NULL,
    end_at DATETIME NOT NULL,
    max_groups INT NOT NULL DEFAULT 0 COMMENT '总团数上限(按开团数计), 0=不限',
    opened_count INT NOT NULL DEFAULT 0 COMMENT '已开团数(原子计数器)',
    expire_hours INT NOT NULL DEFAULT 0 COMMENT '每团有效期(小时)覆盖, 0=用全局配置',
    status TINYINT NOT NULL DEFAULT 1 COMMENT '1 上架 / 0 下架',
    created_at DATETIME,
    updated_at DATETIME,
    KEY idx_gba_sku (sku_id),
    KEY idx_gba_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='拼团限时活动';

ALTER TABLE group_buy ADD COLUMN activity_id BIGINT NOT NULL DEFAULT 0 COMMENT '来源活动, 0=SKU 常开配置';
