package com.example.order.domain.entity;

import java.time.LocalDateTime;
import lombok.Data;

/**
 * 支付单实体，对应表 {@code payment_order}。
 *
 * <p>一个订单可以先后创建多张支付单（换渠道 / 支付失败后重试），未支付的支付单会被复用。
 */
@Data
public class PaymentOrder {

    /** 内部主键 */
    private Long id;

    /** 支付单业务编号，对外唯一标识 */
    private String paymentId;

    /** 关联订单业务编号 */
    private String orderId;

    /** 支付用户UID */
    private String uid;

    /** 支付金额，单位：分 */
    private Long amount;

    /** 支付单状态 {@link com.example.order.domain.enums.PaymentStatus} */
    private Integer status;

    /** 支付渠道 {@link com.example.order.domain.enums.PaymentChannel} */
    private Integer paymentChannel;

    /** 支付渠道流水号，回调成功后回填；唯一 */
    private String transactionId;

    /** 支付截止时间，与订单的支付截止时间一致 */
    private LocalDateTime expireAt;

    /** 支付成功时间 */
    private LocalDateTime paidAt;

    private LocalDateTime createdAt;

    private LocalDateTime updatedAt;
}
