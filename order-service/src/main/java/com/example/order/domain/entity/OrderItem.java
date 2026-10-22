package com.example.order.domain.entity;

import java.time.LocalDateTime;
import lombok.Data;

/**
 * 订单明细实体，对应表 {@code order_item}。
 *
 * <p>下单时对商品与 SKU 做<b>快照</b>（名称 / 单价），避免后续商品改名改价影响历史订单展示。
 */
@Data
public class OrderItem {

    /** 内部主键 */
    private Long id;

    /** 所属订单内部主键（{@code order.id}） */
    private Long orderId;

    /** 商品业务编号 */
    private String productId;

    /** SKU业务编号 */
    private String skuId;

    /** 下单时的商品名称快照 */
    private String productName;

    /** 下单时的SKU名称快照 */
    private String skuName;

    /** 下单时的单价快照，单位：分 */
    private Long unitPrice;

    /** 购买数量 */
    private Integer quantity;

    /** 该明细的小计金额 = 单价 * 数量，单位：分 */
    private Long totalAmount;

    private LocalDateTime createdAt;
}
