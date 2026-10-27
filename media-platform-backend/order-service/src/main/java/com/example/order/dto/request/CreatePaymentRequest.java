package com.example.order.dto.request;

import jakarta.validation.constraints.NotNull;

/**
 * 创建支付单请求体。
 *
 * <p>金额不由客户端传入：服务端取订单的 {@code total_amount}，避免金额被篡改。
 */
public record CreatePaymentRequest(

        @NotNull(message = "支付渠道不能为空")
        Integer paymentChannel
) {
}
