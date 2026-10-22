package com.example.product.dto.response;

import com.example.product.domain.entity.Inventory;
import com.example.product.domain.entity.Product;
import com.example.product.domain.entity.Sku;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * 商品详情：商品本身 + 其下所有未删除的 SKU（含库存快照）。
 */
public record ProductDetailVO(ProductVO product, List<SkuVO> skus) {

    public static ProductDetailVO of(Product product, List<Sku> skus, Map<Long, Inventory> inventoryMap) {
        List<SkuVO> skuViews = new ArrayList<>(skus == null ? 0 : skus.size());
        if (skus != null) {
            for (Sku sku : skus) {
                skuViews.add(SkuVO.from(sku, inventoryMap == null ? null : inventoryMap.get(sku.getId())));
            }
        }
        return new ProductDetailVO(ProductVO.from(product), skuViews);
    }
}
