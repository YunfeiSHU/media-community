package com.example.product.dto.response;

import com.example.product.domain.entity.Product;
import java.time.LocalDateTime;

/**
 * 商品视图对象。
 */
public record ProductVO(

        Long id,
        String productId,
        String uid,
        String name,
        String description,
        String coverUrl,
        Integer productType,
        Integer status,
        LocalDateTime createdAt,
        LocalDateTime updatedAt
) {

    public static ProductVO from(Product product) {
        if (product == null) {
            return null;
        }
        return new ProductVO(
                product.getId(),
                product.getProductId(),
                product.getUid(),
                product.getName(),
                product.getDescription(),
                product.getCoverUrl(),
                product.getProductType(),
                product.getStatus(),
                product.getCreatedAt(),
                product.getUpdatedAt());
    }
}
