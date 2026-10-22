-- ------------------------------------------------------------
-- 商品表：商品本身是什么，以及这个商品属于谁
-- ------------------------------------------------------------
CREATE TABLE `product` (
    id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '商品内部ID',

    product_id VARCHAR(32) NOT NULL COMMENT '商品业务编号',

    uid VARCHAR(32) NOT NULL COMMENT '商品创建者UID',

    name VARCHAR(128) NOT NULL COMMENT '商品名称',

    description VARCHAR(500) DEFAULT NULL COMMENT '商品描述',

    cover_url VARCHAR(512) DEFAULT NULL COMMENT '商品封面',

    product_type TINYINT NOT NULL COMMENT '商品类型 1-实物 2-虚拟',

    status TINYINT NOT NULL DEFAULT 0 COMMENT '商品状态 0-下架 1-上架 2-已删除',

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uk_product_id (product_id),

    KEY idx_uid_status (uid, status),

    KEY idx_type_status (product_type, status)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COMMENT='商品表';

-- ------------------------------------------------------------
-- SKU 表：商品具体卖什么规格，以及具体售价
-- ------------------------------------------------------------
CREATE TABLE `sku` (
    id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT 'SKU内部ID',

    sku_id VARCHAR(32) NOT NULL COMMENT 'SKU业务编号',

    product_id BIGINT NOT NULL COMMENT '所属商品ID(product.id)',

    name VARCHAR(128) NOT NULL COMMENT 'SKU名称',

    price BIGINT NOT NULL COMMENT '售价，单位：分',

    status TINYINT NOT NULL DEFAULT 1 COMMENT 'SKU状态 0-禁用 1-启用 2-已删除',

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uk_sku_id (sku_id),

    KEY idx_product_id (product_id),

    KEY idx_product_status (product_id, status)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COMMENT='商品SKU表';

-- ------------------------------------------------------------
-- 库存表：SKU 当前有多少可用库存，以及多少库存已经被订单预占
-- ------------------------------------------------------------
CREATE TABLE `inventory` (
    id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '库存记录ID',

    sku_id BIGINT NOT NULL COMMENT 'SKU内部ID(sku.id)',

    available_stock INT NOT NULL DEFAULT 0 COMMENT '可用库存',

    locked_stock INT NOT NULL DEFAULT 0 COMMENT '已锁定库存',

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uk_sku_id (sku_id)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COMMENT='SKU库存表';
