package com.example.product.api.dto;

import com.example.product.domain.entity.Product;
import java.io.Serial;
import java.io.Serializable;
import java.time.LocalDateTime;
import lombok.Data;

/**
 * 商品 RPC 传输对象。
 */
@Data
public class ProductDTO implements Serializable {

    @Serial
    private static final long serialVersionUID = 1L;

    /** 商品业务编号 */
    private String productId;

    /** 创建者UID */
    private String uid;

    private String name;

    private String description;

    private String coverUrl;

    /** 商品类型 1-实物 2-虚拟 */
    private Integer productType;

    /** 商品状态 0-下架 1-上架 2-已删除 */
    private Integer status;

    private LocalDateTime createdAt;

    public static ProductDTO of(Product product) {
        ProductDTO dto = new ProductDTO();
        dto.setProductId(product.getProductId());
        dto.setUid(product.getUid());
        dto.setName(product.getName());
        dto.setDescription(product.getDescription());
        dto.setCoverUrl(product.getCoverUrl());
        dto.setProductType(product.getProductType());
        dto.setStatus(product.getStatus());
        dto.setCreatedAt(product.getCreatedAt());
        return dto;
    }
}
