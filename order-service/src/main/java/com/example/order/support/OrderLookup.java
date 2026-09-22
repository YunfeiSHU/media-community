package com.example.order.support;

import com.example.order.common.BizException;
import com.example.order.common.ErrorCode;
import com.example.order.common.Identifiers;
import com.example.order.domain.entity.Order;
import com.example.order.mapper.OrderMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

/**
 * 订单的统一查询入口：负责「内部ID 或 业务编号」的解析与「不存在 / 不属于当前用户」的判定。
 */
@Component
@RequiredArgsConstructor
public class OrderLookup {

    private final OrderMapper orderMapper;

    /**
     * 订单必须存在（内部使用，如支付回调按 orderId 反查订单）。
     */
    public Order require(String rawId) {
        Order order = find(rawId);
        if (order == null) {
            throw new BizException(ErrorCode.ORDER_NOT_FOUND, "订单不存在: " + rawId);
        }
        return order;
    }

    /**
     * 订单必须存在且属于该用户。
     *
     * <p>他人的订单一律按「不存在」返回 404，而不是 403：避免通过错误码差异探测他人订单是否存在。
     */
    public Order requireMine(String rawId, String uid) {
        if (rawId == null || rawId.isBlank()) {
            throw new BizException(ErrorCode.INVALID_PARAM, "订单标识不能为空");
        }
        Long internalId = Identifiers.parseInternalId(rawId);
        Order order = internalId == null
                ? orderMapper.selectByOrderIdAndUid(rawId, uid)
                : orderMapper.selectById(internalId);
        if (order == null || !uid.equals(order.getUid())) {
            throw new BizException(ErrorCode.ORDER_NOT_FOUND, "订单不存在: " + rawId);
        }
        return order;
    }

    /** 按「内部ID 或 业务编号」查询订单，查不到返回 null */
    public Order find(String rawId) {
        if (rawId == null || rawId.isBlank()) {
            return null;
        }
        Long internalId = Identifiers.parseInternalId(rawId);
        return internalId == null ? orderMapper.selectByOrderId(rawId) : orderMapper.selectById(internalId);
    }
}
