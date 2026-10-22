package com.example.product.support;

import com.example.product.common.BizException;
import com.example.product.common.ErrorCode;
import com.example.product.domain.entity.Product;
import com.example.product.domain.entity.Sku;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

/**
 * 商品归属校验：写操作只允许商品创建者（{@code product.uid}）执行。
 *
 * <p>鉴权（是否登录）由网关的 JWT 校验保证，授权（能不能动这条数据）在这里判定：
 * 商品归属以 {@code product.uid} 为唯一依据，不接受请求体 / 查询参数里的用户标识。
 *
 * <p>只读查询（商品详情、SKU 详情、价格、库存）不校验归属，保持对外公开。
 *
 * <p>仅用于 HTTP 接入层；Dubbo RPC 入口（order-service 的库存预占/确认/释放等）属于内部服务调用，
 * 没有用户上下文，不做归属校验。
 */
@Component
@RequiredArgsConstructor
public class ProductOwnership {

    private final EntityLookup entityLookup;

    /**
     * 校验商品归属：{@code id} 支持内部主键或业务编号。
     *
     * @return 已确认归属的商品
     * @throws BizException 商品不存在（40401）或非本人创建（40301）
     */
    public Product requireProductOwner(String id, String uid) {
        Product product = entityLookup.requireProduct(id);
        requireOwner(product, uid);
        return product;
    }

    /**
     * 校验 SKU 归属：按 SKU 反查所属商品，再比对创建者。
     *
     * @return 已确认归属的 SKU
     * @throws BizException SKU/商品不存在（40402/40401）或非本人创建（40301）
     */
    public Sku requireSkuOwner(String id, String uid) {
        Sku sku = entityLookup.requireSku(id);
        Product product = entityLookup.requireProduct(String.valueOf(sku.getProductId()));
        requireOwner(product, uid);
        return sku;
    }

    /**
     * 判断商品是否由指定用户创建（供需要分支处理的调用方使用）。
     */
    public boolean isOwner(Product product, String uid) {
        return product != null && uid != null && uid.equals(product.getUid());
    }

    private void requireOwner(Product product, String uid) {
        if (!isOwner(product, uid)) {
            // 商品详情本身是公开信息，因此这里直接返回 403，不回退成 404
            throw new BizException(ErrorCode.FORBIDDEN, "无权操作他人商品: " + product.getProductId());
        }
    }
}
