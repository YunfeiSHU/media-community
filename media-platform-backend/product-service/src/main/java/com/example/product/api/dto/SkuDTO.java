package com.example.product.api.dto;

import com.example.product.domain.entity.Sku;
import java.io.Serial;
import java.io.Serializable;
import java.time.LocalDateTime;
import lombok.Data;

/**
 * SKU RPC 传输对象，对外统一使用业务编号。
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

    public static SkuDTO of(Sku sku, String productCode) {
        SkuDTO dto = new SkuDTO();
        dto.setSkuId(sku.getSkuId());
        dto.setProductId(productCode);
        dto.setName(sku.getName());
        dto.setPrice(sku.getPrice());
        dto.setStatus(sku.getStatus());
        dto.setCreatedAt(sku.getCreatedAt());
        return dto;
    }
}
