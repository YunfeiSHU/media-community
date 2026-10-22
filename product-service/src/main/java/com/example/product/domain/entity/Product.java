package com.example.product.domain.entity;

import java.time.LocalDateTime;
import lombok.Data;

/**
 * 商品实体，对应表 {@code product}。
 */
@Data
public class Product {

    /** 内部主键 */
    private Long id;

    /** 商品业务编号 */
    private String productId;

    /** 创建者UID */
    private String uid;

    private String name;

    private String description;

    private String coverUrl;

    /** 商品类型 {@link com.example.product.domain.enums.ProductType} */
    private Integer productType;

    /** 商品状态 {@link com.example.product.domain.enums.ProductStatus} */
    private Integer status;

    private LocalDateTime createdAt;

    private LocalDateTime updatedAt;
}
