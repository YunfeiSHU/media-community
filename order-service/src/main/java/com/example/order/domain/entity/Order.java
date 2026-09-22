package com.example.order.domain.entity;

import java.time.LocalDateTime;
import lombok.Data;

/**
 * 订单实体，对应表 {@code order}。
 */
@Data
public class Order {

    /** 内部主键 */
    private Long id;

    /** 订单业务编号，对外唯一标识 */
    private String orderId;

    /** 下单用户UID */
    private String uid;

    /** 订单状态 {@link com.example.order.domain.enums.OrderStatus} */
    private Integer status;

    /** 订单总金额，单位：分 */
    private Long totalAmount;

    /** 支付截止时间，超过该时间未支付则被定时任务取消 */
    private LocalDateTime expireAt;

    /** 支付成功时间 */
    private LocalDateTime paidAt;

    /** 完成时间 */
    private LocalDateTime completedAt;

    /** 取消时间 */
    private LocalDateTime cancelledAt;

    private LocalDateTime createdAt;

    private LocalDateTime updatedAt;
}
