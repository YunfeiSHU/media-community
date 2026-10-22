package com.example.order.service;

import com.example.order.dto.request.CreatePaymentRequest;
import com.example.order.dto.request.PaymentCallbackRequest;
import com.example.order.dto.response.PaymentVO;

/**
 * 支付域服务：创建支付单 + 处理渠道异步回调。
 */
public interface PaymentService {

    /**
     * 创建支付单。
     *
     * <p>幂等：订单若已存在未支付的支付单则直接复用，避免用户反复点击生成多张支付单。
     */
    PaymentVO createPayment(String uid, String orderId, CreatePaymentRequest request);

    /**
     * 处理支付渠道异步回调：验签 → 留痕 → 支付单置为已支付 → 驱动订单状态机。
     */
    void handleCallback(PaymentCallbackRequest request);
}
