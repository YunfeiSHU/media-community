package com.example.product.dto.response;

import com.example.product.domain.entity.Product;
import com.example.product.domain.enums.ProductStatus;
import java.time.LocalDateTime;

/**
 * 商品视图对象。
 *
 * <p>列表页需要「最低价 / SKU 数 / 状态文案」，否则前端只能硬编码 0/1/2 或再发一次详情请求。
 * 详情页也能用同一结构：{@code skus} 已加载时直接就地算聚合，不必多查一次库。
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
        /** 状态中文名：下架 / 上架 / 已删除——避免前端硬编码数字 */
        String statusName,
        /** 最低价（分）；没有可用 SKU 时为 null */
        Long minPrice,
        /** 未删除的 SKU 数量 */
        Long skuCount,
        LocalDateTime createdAt,
        LocalDateTime updatedAt
) {

    public static ProductVO from(Product product) {
        return from(product, null, 0L);
    }

    public static ProductVO from(Product product, ProductSkuStat stat) {
        if (stat == null) {
            return from(product, null, 0L);
        }
        return from(product, stat.getMinPrice(), stat.getSkuCount() == null ? 0L : stat.getSkuCount());
    }

    public static ProductVO from(Product product, Long minPrice, Long skuCount) {
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
                statusName(product.getStatus()),
                minPrice,
                skuCount == null ? 0L : skuCount,
                product.getCreatedAt(),
                product.getUpdatedAt());
    }

    private static String statusName(Integer status) {
        if (status == null) {
            return "未知";
        }
        for (ProductStatus candidate : ProductStatus.values()) {
            if (candidate.is(status)) {
                return switch (candidate) {
                    case OFF_SHELF -> "下架";
                    case ON_SHELF -> "上架";
                    case DELETED -> "已删除";
                };
            }
        }
        return "未知";
    }
}
