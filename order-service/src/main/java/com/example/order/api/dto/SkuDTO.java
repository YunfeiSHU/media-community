package com.example.order.api.dto;

import java.io.Serial;
import java.io.Serializable;
import java.time.LocalDateTime;
import lombok.Data;

/**
 * SKU RPC 传输对象（与 product-service 的 {@code com.example.product.api.dto.SkuDTO} 字段一致）。
 */
@Data
public class SkuDTO implements Serializable {

    @Serial
    private static final long serialVersionUID = 1L;

    /** SKU业务编号 */
    private String skuId;

    /** 所属商品业务编号 */
    private String productId;

    private String name;

    /** 售价，单位：分 */
    private Long price;

    /** SKU状态 0-禁用 1-启用 2-已删除 */
    private Integer status;

    private LocalDateTime createdAt;
}
