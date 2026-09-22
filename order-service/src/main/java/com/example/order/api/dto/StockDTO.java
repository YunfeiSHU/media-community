package com.example.order.api.dto;

import java.io.Serial;
import java.io.Serializable;
import lombok.Data;

/**
 * 库存 RPC 传输对象（与 product-service 的 {@code com.example.product.api.dto.StockDTO} 字段一致）。
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
}
