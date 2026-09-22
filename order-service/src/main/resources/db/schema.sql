-- ============================================================
-- order-service 建表脚本（MySQL 8，手工执行）
-- 库：order_db（与 product-service 的 product_db 位于同一 MySQL 实例，但分属两个服务）
--
-- 说明：与《设计文档.md》的表结构保持一致，两处按实际 RPC 契约做了修正：
--   1) order_item.product_id / sku_id 由 BIGINT 改为 VARCHAR(32)
--      原因：product-service 的 RPC 契约（SkuDTO.productId / SkuDTO.skuId）对外只暴露业务编号，
--            order-service 拿不到商品/SKU 的内部自增 ID
--   2) payment_record 的索引 idx_payment_no 修正为 idx_payment_id（原 DDL 引用了不存在的列）
-- ============================================================

CREATE TABLE IF NOT EXISTS `order` (
    id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '订单内部ID',
    order_id VARCHAR(32) NOT NULL COMMENT '订单业务编号',
    uid VARCHAR(32) NOT NULL COMMENT '下单用户UID',
    status TINYINT NOT NULL DEFAULT 0 COMMENT '订单状态 0-待支付 1-已支付 2-已完成 3-已取消',
    total_amount BIGINT NOT NULL COMMENT '订单总金额，单位：分',
    expire_at DATETIME NULL COMMENT '支付截止时间',
    paid_at DATETIME NULL COMMENT '支付成功时间',
    completed_at DATETIME NULL COMMENT '完成时间',
    cancelled_at DATETIME NULL COMMENT '取消时间',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_order_id (order_id),
    KEY idx_uid_created_at (uid, created_at),
    KEY idx_status_expire_at (status, expire_at)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COMMENT='订单表';

CREATE TABLE IF NOT EXISTS `order_item` (
    id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '明细内部ID',
    order_id BIGINT NOT NULL COMMENT '所属订单内部ID（order.id）',
    product_id VARCHAR(32) NOT NULL COMMENT '商品业务编号',
    sku_id VARCHAR(32) NOT NULL COMMENT 'SKU业务编号',
    product_name VARCHAR(128) NOT NULL COMMENT '下单时的商品名称快照',
    sku_name VARCHAR(128) NOT NULL COMMENT '下单时的SKU名称快照',
    unit_price BIGINT NOT NULL COMMENT '下单时的单价快照，单位：分',
    quantity INT NOT NULL COMMENT '购买数量',
    total_amount BIGINT NOT NULL COMMENT '小计金额，单位：分',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_order_id (order_id),
    KEY idx_product_id (product_id),
    KEY idx_sku_id (sku_id)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COMMENT='订单明细表';

CREATE TABLE IF NOT EXISTS `payment_order` (
    id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '支付单内部ID',
    payment_id VARCHAR(32) NOT NULL COMMENT '支付单业务编号',
    order_id VARCHAR(32) NOT NULL COMMENT '关联订单业务编号',
    uid VARCHAR(32) NOT NULL COMMENT '支付用户UID',
    amount BIGINT NOT NULL COMMENT '支付金额，单位：分',
    status TINYINT NOT NULL DEFAULT 0 COMMENT '支付单状态 0-待支付 1-已支付 2-已关闭 3-支付失败',
    payment_channel TINYINT NOT NULL COMMENT '支付渠道 1-微信 2-支付宝 3-余额',
    transaction_id VARCHAR(128) NULL COMMENT '支付渠道流水号',
    expire_at DATETIME NULL COMMENT '支付截止时间',
    paid_at DATETIME NULL COMMENT '支付成功时间',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_payment_id (payment_id),
    UNIQUE KEY uk_transaction_id (transaction_id),
    KEY idx_order_id (order_id),
    KEY idx_uid_created_at (uid, created_at)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COMMENT='支付单表';

CREATE TABLE IF NOT EXISTS `payment_record` (
    id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '流水内部ID',
    payment_id VARCHAR(32) NOT NULL COMMENT '关联支付单业务编号',
    transaction_id VARCHAR(128) NULL COMMENT '支付渠道流水号',
    payment_channel TINYINT NOT NULL COMMENT '支付渠道',
    callback_status TINYINT NOT NULL COMMENT '回调处理结果 1-成功 2-失败',
    callback_data TEXT NULL COMMENT '回调原始报文',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_payment_id (payment_id),
    KEY idx_transaction_id (transaction_id)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COMMENT='支付回调流水表';
