package com.example.order.api;

import com.example.order.api.dto.StockDTO;

/**
 * 库存域 Dubbo 服务（消费方契约副本，全限定名与 product-service 保持一致）。
 *
 * <p>变更类方法返回布尔值而不抛异常，便于调用方在分布式场景下做补偿判断：
 * 返回 {@code false} 表示 SKU 不存在 / 库存记录缺失 / 库存不足，业务未发生变更。
 */
public interface StockRpcService {

    /**
     * 查询库存。
     *
     * @param skuId SKU业务编号
     * @return 库存信息，不存在返回 null
     */
    StockDTO getStock(String skuId);

    /**
     * 扣减库存（订单预占：可用库存 → 锁定库存）。
     *
     * @return 是否扣减成功
     */
    boolean reserveStock(String skuId, int quantity);

    /**
     * 确认扣减（订单支付成功：消耗锁定库存）。
     *
     * @return 是否确认成功
     */
    boolean confirmStock(String skuId, int quantity);

    /**
     * 释放库存（订单取消/超时：锁定库存 → 可用库存）。
     *
     * @return 是否释放成功
     */
    boolean releaseStock(String skuId, int quantity);
}
