package com.example.product.api;

import com.example.product.api.dto.ProductDTO;
import com.example.product.api.dto.SkuDTO;
import java.util.List;

/**
 * 商品域 Dubbo 服务，供订单/营销等上游服务调用。
 *
 * <p>查询语义：查不到时返回 {@code null}（{@link #getSkus} 返回空集合），不抛异常。
 */
public interface ProductRpcService {

    /**
     * 查询商品。
     *
     * @param productId 商品业务编号
     * @return 商品信息，不存在或已删除返回 null
     */
    ProductDTO getProduct(String productId);

    /**
     * 查询单个 SKU。
     *
     * @param skuId SKU业务编号
     * @return SKU信息，不存在或已删除返回 null
     */
    SkuDTO getSku(String skuId);

    /**
     * 查询商品下的全部可用 SKU。
     *
     * @param productId 商品业务编号
     * @return SKU列表，商品不存在时返回空集合
     */
    List<SkuDTO> getSkus(String productId);
}
