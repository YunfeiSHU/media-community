package com.example.product.api.dto;

import com.example.product.domain.entity.Inventory;
import java.io.Serial;
import java.io.Serializable;
import lombok.Data;

/**
 * 库存 RPC 传输对象。
 */
@Data
public class StockDTO implements Serializable {

    @Serial
    private static final long serialVersionUID = 1L;

    /** SKU业务编号 */
    private String skuId;

    /** 可用库存 */
    private Integer availableStock;

    /** 已锁定库存 */
    private Integer lockedStock;

    public static StockDTO of(String skuId, Inventory inventory) {
        StockDTO dto = new StockDTO();
        dto.setSkuId(skuId);
        dto.setAvailableStock(inventory.getAvailableStock());
        dto.setLockedStock(inventory.getLockedStock());
        return dto;
    }
}
