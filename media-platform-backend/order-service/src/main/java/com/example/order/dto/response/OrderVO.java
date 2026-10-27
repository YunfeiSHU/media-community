package com.example.order.dto.response;

import com.example.order.domain.entity.Order;
import com.example.order.domain.entity.OrderItem;
import com.example.order.domain.enums.OrderStatus;
import java.time.LocalDateTime;
import java.util.List;

/**
 * 订单响应：订单主信息 + 明细列表。
 *
 * @param status     订单状态码
 * @param statusName 订单状态描述，便于前端直接展示
 */
public record OrderVO(Long id,
                      String orderId,
                      String uid,
                      Integer status,
                      String statusName,
                      Long totalAmount,
                      LocalDateTime expireAt,
                      LocalDateTime paidAt,
                      LocalDateTime completedAt,
                      LocalDateTime cancelledAt,
                      LocalDateTime createdAt,
                      LocalDateTime updatedAt,
                      List<OrderItemVO> items) {

    public static OrderVO of(Order order, List<OrderItem> items) {
        if (order == null) {
            return null;
        }
        return new OrderVO(order.getId(), order.getOrderId(), order.getUid(), order.getStatus(),
                OrderStatus.of(order.getStatus()).getDescription(), order.getTotalAmount(),
                order.getExpireAt(), order.getPaidAt(), order.getCompletedAt(), order.getCancelledAt(),
                order.getCreatedAt(), order.getUpdatedAt(),
                items == null ? List.of() : items.stream().map(OrderItemVO::from).toList());
    }
}
