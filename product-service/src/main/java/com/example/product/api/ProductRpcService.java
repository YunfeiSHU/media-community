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
     * 查询商品下的全部 SKU。
     *
     * <p>注意语义是“未删除的 SKU”，<b>包含状态为禁用的 SKU</b>（SQL 条件是 {@code status <> DELETED}）；
     * 下单链路会自行校验 {@code status == ENABLED} 与库存，所以这里不做过濾。
     * 调用方不要把它当作“可售 SKU 列表”使用。
     *
     * @param productId 商品业务编号
     * @return SKU 列表；商品不存在或已删除时返回空集合（不抛异常）
     */
    List<SkuDTO> getSkus(String productId);
}
