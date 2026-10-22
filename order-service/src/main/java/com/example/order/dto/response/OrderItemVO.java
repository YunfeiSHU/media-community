package com.example.order.dto.response;

import com.example.order.domain.entity.OrderItem;

/**
 * 订单明细响应：下单时的商品 / SKU 快照，不随商品后续变更而改变。
 */
public record OrderItemVO(Long id,
                          String productId,
                          String skuId,
                          String productName,
                          String skuName,
                          Long unitPrice,
                          Integer quantity,
                          Long totalAmount) {

    public static OrderItemVO from(OrderItem item) {
        if (item == null) {
            return null;
        }
        return new OrderItemVO(item.getId(), item.getProductId(), item.getSkuId(), item.getProductName(),
                item.getSkuName(), item.getUnitPrice(), item.getQuantity(), item.getTotalAmount());
    }
}
