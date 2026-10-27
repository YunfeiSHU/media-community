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

    /**
     * 模拟支付（仅本地/演示启用）。
     *
     * <p>生产链路必须有真实渠道回调才会置为已支付；而本仓库没有接真实渠道，
     * 如果不提供这个入口，用户在页面上点完“支付”订单永远停在待支付，
     * 商城闭环就是断的。实现上复用与回调相同的「支付单 markPaid + 订单 markPaid」，
     * 但不做验签——因为调用者本身就是网关校验过的订单所有者。
     *
     * @param uid     当前登录用户，必须是订单所有者
     * @param orderId 订单业务编号
     */
    PaymentVO mockPay(String uid, String orderId);
}
