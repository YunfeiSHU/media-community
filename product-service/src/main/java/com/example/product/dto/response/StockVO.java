package com.example.product.dto.response;

import com.example.product.domain.entity.Inventory;
import java.time.LocalDateTime;

/**
 * 库存视图。
 */
public record StockVO(String skuId, Integer availableStock, Integer lockedStock,
                      LocalDateTime updatedAt) {

    public static StockVO from(String skuId, Inventory inventory) {
        return new StockVO(skuId,
                inventory.getAvailableStock(),
                inventory.getLockedStock(),
                inventory.getUpdatedAt());
    }
}
