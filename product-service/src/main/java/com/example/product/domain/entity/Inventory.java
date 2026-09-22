package com.example.product.domain.entity;

import java.time.LocalDateTime;
import lombok.Data;

/**
 * SKU 库存实体，对应表 {@code inventory}。一个 SKU 只有一条库存记录。
 */
@Data
public class Inventory {

    private Long id;

    /** SKU内部ID */
    private Long skuId;

    /** 可用库存 */
    private Integer availableStock;

    /** 已锁定库存（订单预占，未支付确认） */
    private Integer lockedStock;

    private LocalDateTime createdAt;

    private LocalDateTime updatedAt;
}
