package com.example.product.api.impl;

import com.example.product.api.ProductRpcService;
import com.example.product.api.dto.ProductDTO;
import com.example.product.api.dto.SkuDTO;
import com.example.product.domain.entity.Product;
import com.example.product.domain.entity.Sku;
import com.example.product.domain.enums.ProductStatus;
import com.example.product.domain.enums.SkuStatus;
import com.example.product.mapper.SkuMapper;
import com.example.product.support.EntityLookup;
import java.util.List;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.apache.dubbo.config.annotation.DubboService;
import org.springframework.transaction.annotation.Transactional;

/**
 * 商品域 Dubbo 服务实现。
 */
@Slf4j
@DubboService(version = "1.0.0")
@RequiredArgsConstructor
public class ProductRpcServiceImpl implements ProductRpcService {

    private final EntityLookup entityLookup;

    private final SkuMapper skuMapper;

    @Override
    @Transactional(readOnly = true)
    public ProductDTO getProduct(String productId) {
        Product product = findActiveProduct(productId);
        if (product == null) {
            log.warn("[RPC] getProduct 未命中: productId={}", productId);
            return null;
        }
        log.debug("[RPC] getProduct 成功: productId={}", productId);
        return ProductDTO.of(product);
    }

    @Override
    @Transactional(readOnly = true)
    public SkuDTO getSku(String skuId) {
        Sku sku = entityLookup.findSku(skuId);
        if (sku == null || SkuStatus.DELETED.is(sku.getStatus())) {
            log.warn("[RPC] getSku 未命中: skuId={}", skuId);
            return null;
        }
        Product product = entityLookup.findProduct(String.valueOf(sku.getProductId()));
        log.debug("[RPC] getSku 成功: skuId={}", skuId);
        return SkuDTO.of(sku, product == null ? null : product.getProductId());
    }

    @Override
    @Transactional(readOnly = true)
    public List<SkuDTO> getSkus(String productId) {
        Product product = findActiveProduct(productId);
        if (product == null) {
            log.warn("[RPC] getSkus 未命中: productId={}", productId);
            return List.of();
        }
        List<SkuDTO> skus = skuMapper.selectByProductId(product.getId()).stream()
                .map(sku -> SkuDTO.of(sku, product.getProductId()))
                .toList();
        log.debug("[RPC] getSkus 成功: productId={}, size={}", productId, skus.size());
        return skus;
    }

    private Product findActiveProduct(String id) {
        Product product = entityLookup.findProduct(id);
        if (product == null || ProductStatus.DELETED.is(product.getStatus())) {
            return null;
        }
        return product;
    }
}
