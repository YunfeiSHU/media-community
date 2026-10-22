package com.example.order.dto.response;

import com.example.order.domain.entity.PaymentOrder;
import com.example.order.domain.enums.PaymentChannel;
import com.example.order.domain.enums.PaymentStatus;
import java.time.LocalDateTime;

/**
 * 支付单响应。
 *
 * <p>真实渠道会在这里额外返回「拉起收银台需要的参数」（prepayId / payUrl 等）；
 * 本项目不接真实渠道，渠道参数由调用方按约定自行模拟。
 *
 * @param paymentChannel     支付渠道码
 * @param paymentChannelName 支付渠道描述
 * @param status             支付单状态码
 * @param statusName         支付单状态描述
 */
public record PaymentVO(Long id,
                        String paymentId,
                        String orderId,
                        String uid,
                        Long amount,
                        Integer status,
                        String statusName,
                        Integer paymentChannel,
                        String paymentChannelName,
                        String transactionId,
                        LocalDateTime expireAt,
                        LocalDateTime paidAt,
                        LocalDateTime createdAt) {

    public static PaymentVO from(PaymentOrder payment) {
        if (payment == null) {
            return null;
        }
        return new PaymentVO(payment.getId(), payment.getPaymentId(), payment.getOrderId(), payment.getUid(),
                payment.getAmount(),
                payment.getStatus(), PaymentStatus.of(payment.getStatus()).getDescription(),
                payment.getPaymentChannel(), PaymentChannel.of(payment.getPaymentChannel()).getDescription(),
                payment.getTransactionId(), payment.getExpireAt(), payment.getPaidAt(), payment.getCreatedAt());
    }
}
