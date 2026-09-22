package com.example.product.api;

import com.example.product.api.dto.ProductDTO;
import com.example.product.api.dto.SkuDTO;
import java.util.List;

/**
 * 商品域 Dubbo 服务（消费方契约副本）。
 *
 * <p>该接口的<b>全限定名、方法签名必须与 product-service 暴露的实现完全一致</b>，否则 Dubbo 无法完成
 * 服务寻址与序列化。order-service 只依赖契约，不依赖 product-service 的任何实现类（实体 / Service）。
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
