package com.example.product.dto.response;

import com.example.product.domain.entity.Inventory;
import com.example.product.domain.entity.Sku;
import java.time.LocalDateTime;

/**
 * SKU 视图对象（含当前库存快照）。
 */
public record SkuVO(

        Long id,
        String skuId,
        Long productId,
        String name,
        /** 售价，单位：分 */
        Long price,
        Integer status,
        Integer availableStock,
        Integer lockedStock,
        LocalDateTime createdAt,
        LocalDateTime updatedAt
) {

    public static SkuVO from(Sku sku, Inventory inventory) {
        if (sku == null) {
            return null;
        }
        return new SkuVO(
                sku.getId(),
                sku.getSkuId(),
                sku.getProductId(),
                sku.getName(),
                sku.getPrice(),
                sku.getStatus(),
                inventory == null ? 0 : inventory.getAvailableStock(),
                inventory == null ? 0 : inventory.getLockedStock(),
                sku.getCreatedAt(),
                sku.getUpdatedAt());
    }
}
