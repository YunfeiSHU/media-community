package com.example.product.domain.entity;

import java.time.LocalDateTime;
import lombok.Data;

/**
 * SKU 实体，对应表 {@code sku}。
 */
@Data
public class Sku {

    /** 内部主键 */
    private Long id;

    /** SKU业务编号 */
    private String skuId;

    /** 所属商品内部ID */
    private Long productId;

    private String name;

    /** 售价，单位：分 */
    private Long price;

    /** SKU状态 {@link com.example.product.domain.enums.SkuStatus} */
    private Integer status;

    private LocalDateTime createdAt;

    private LocalDateTime updatedAt;
}
