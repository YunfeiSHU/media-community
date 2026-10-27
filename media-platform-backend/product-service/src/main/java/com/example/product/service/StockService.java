package com.example.product.service;

import com.example.product.dto.response.StockVO;

/**
 * 库存域服务。
 *
 * <p>库存模型：
 * <pre>
 *   可用库存 available_stock ：可以直接下单的数量
 *   锁定库存 locked_stock    ：已被订单预占、尚未支付确认的数量
 * </pre>
 *
 * <p>典型交易链路：下单 → {@link #reserveStock} 扣减可用并锁定 → 支付成功 {@link #confirmStock}
 * 扣掉锁定；支付超时/取消 → {@link #releaseStock} 把锁定库存还回可用库存。
 * 入库补货走 {@link #increaseStock}。
 */
public interface StockService {

    /** 查询库存 */
    StockVO getStock(String skuId);

    /** 增加库存（补货入库） */
    StockVO increaseStock(String skuId, int quantity);

    /** 扣减库存（订单预占：可用 → 锁定） */
    StockVO reserveStock(String skuId, int quantity);

    /** 释放库存（订单取消：锁定 → 可用） */
    StockVO releaseStock(String skuId, int quantity);

    /** 确认扣减（订单支付成功：真正消耗锁定库存） */
    StockVO confirmStock(String skuId, int quantity);
}
