package com.example.product.dto.response;

import com.example.product.domain.entity.Inventory;
import com.example.product.domain.entity.Sku;
import java.time.LocalDateTime;

/**
 * SKU 视图对象（含当前库存快照）。
 *
 * <p>字段命名规则（与 {@link ProductVO} 对齐）：
 * <ul>
 *     <li>{@code skuId}：对外的 SKU 业务编号，形如 {@code S...}，接口入参就用它；</li>
 *     <li>{@code id} / {@code productDbId}：库内自增主键与外键，仅调试/管理端可见。
 *         早期把这个外键叫 {@code productId}，与 {@code ProductVO.productId}（业务编号）同名不同义，
 *         现已改名为 {@code productDbId} 以免用错。</li>
 * </ul>
 */
public record SkuVO(

        /** SKU 内部自增主键（不是业务编号） */
        Long id,
        /** SKU 业务编号（对客标识） */
        String skuId,
        /** 所属商品的<b>内部</b>主键（不是 productId 业务编号） */
        Long productDbId,
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
