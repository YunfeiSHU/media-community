package com.example.order.domain.entity;

import java.time.LocalDateTime;
import lombok.Data;

/**
 * 支付回调流水实体，对应表 {@code payment_record}。
 *
 * <p>只追加不修改：每一次渠道回调（无论成功、失败、重复）都留一条记录，用于对账与问题排查。
 */
@Data
public class PaymentRecord {

    /** 内部主键 */
    private Long id;

    /** 关联支付单业务编号 */
    private String paymentId;

    /** 支付渠道流水号 */
    private String transactionId;

    /** 支付渠道 {@link com.example.order.domain.enums.PaymentChannel} */
    private Integer paymentChannel;

    /** 回调结果 {@link com.example.order.domain.enums.CallbackStatus} */
    private Integer callbackStatus;

    /** 回调原始报文（JSON），便于事后追溯 */
    private String callbackData;

    private LocalDateTime createdAt;
}
