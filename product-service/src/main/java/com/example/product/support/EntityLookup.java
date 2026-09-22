package com.example.product.support;

import com.example.product.common.BizException;
import com.example.product.common.ErrorCode;
import com.example.product.common.Identifiers;
import com.example.product.domain.entity.Product;
import com.example.product.domain.entity.Sku;
import com.example.product.domain.enums.ProductStatus;
import com.example.product.domain.enums.SkuStatus;
import com.example.product.mapper.ProductMapper;
import com.example.product.mapper.SkuMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

/**
 * 商品 / SKU 的统一查询入口：负责「内部ID 或 业务编号」的解析与「不存在 / 已删除」的判定。
 */
@Component
@RequiredArgsConstructor
public class EntityLookup {

    private final ProductMapper productMapper;

    private final SkuMapper skuMapper;

    /** 商品必须存在且未删除 */
    public Product requireProduct(String id) {
        Product product = findProduct(id);
        if (product == null || ProductStatus.DELETED.is(product.getStatus())) {
            throw new BizException(ErrorCode.PRODUCT_NOT_FOUND, "商品不存在: " + id);
        }
        return product;
    }

    /** SKU 必须存在且未删除 */
    public Sku requireSku(String id) {
        Sku sku = findSku(id);
        if (sku == null || SkuStatus.DELETED.is(sku.getStatus())) {
            throw new BizException(ErrorCode.SKU_NOT_FOUND, "SKU不存在: " + id);
        }
        return sku;
    }

    /** 商品可以不存在 */
    public Product findProduct(String id) {
        requireNotBlank(id, "商品标识不能为空");
        Long internalId = Identifiers.parseInternalId(id);
        return internalId == null ? productMapper.selectByProductId(id) : productMapper.selectById(internalId);
    }

    /** SKU 可以不存在 */
    public Sku findSku(String id) {
        requireNotBlank(id, "SKU标识不能为空");
        Long internalId = Identifiers.parseInternalId(id);
        return internalId == null ? skuMapper.selectBySkuId(id) : skuMapper.selectById(internalId);
    }

    private void requireNotBlank(String id, String message) {
        if (id == null || id.isBlank()) {
            throw new BizException(ErrorCode.INVALID_PARAM, message);
        }
    }
}
