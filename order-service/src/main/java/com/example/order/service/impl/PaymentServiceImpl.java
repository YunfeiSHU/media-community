package com.example.order.service.impl;

import com.example.order.common.BizException;
import com.example.order.common.ErrorCode;
import com.example.order.common.IdGenerator;
import com.example.order.domain.entity.Order;
import com.example.order.domain.entity.PaymentOrder;
import com.example.order.domain.enums.CallbackStatus;
import com.example.order.domain.enums.OrderStatus;
import com.example.order.domain.enums.PaymentChannel;
import com.example.order.domain.enums.PaymentStatus;
import com.example.order.dto.request.CreatePaymentRequest;
import com.example.order.dto.request.PaymentCallbackRequest;
import com.example.order.dto.response.PaymentVO;
import com.example.order.mapper.PaymentOrderMapper;
import com.example.order.service.OrderService;
import com.example.order.service.PaymentService;
import com.example.order.support.OrderLookup;
import com.example.order.support.PaymentRecordWriter;
import com.example.order.support.PaymentSigner;
import java.time.LocalDateTime;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * 支付域实现。
 *
 * <p>处理顺序固定为：<b>验签 → 幂等判定 → 金额/流水号校验 → 状态流转 → 驱动订单状态机 → 留痕</b>。
 * 状态流转全部使用条件更新，因此渠道的重复回调（未收到应答会重试）天然幂等。
 */
@Slf4j
@Service
@RequiredArgsConstructor
public class PaymentServiceImpl implements PaymentService {

    private final PaymentOrderMapper paymentOrderMapper;

    private final PaymentRecordWriter paymentRecordWriter;

    private final PaymentSigner paymentSigner;

    private final OrderLookup orderLookup;

    private final OrderService orderService;

    @Override
    @Transactional(rollbackFor = Exception.class)
    public PaymentVO createPayment(String uid, String orderId, CreatePaymentRequest request) {
        String currentUid = requireUid(uid);
        PaymentChannel channel = PaymentChannel.of(request.paymentChannel());

        Order order = orderLookup.requireMine(orderId, currentUid);
        if (!OrderStatus.WAIT_PAY.is(order.getStatus())) {
            throw new BizException(ErrorCode.ORDER_STATUS_CONFLICT, "订单当前状态不可支付: "
                    + OrderStatus.of(order.getStatus()).getDescription());
        }
        LocalDateTime now = LocalDateTime.now();
        if (order.getExpireAt() != null && !order.getExpireAt().isAfter(now)) {
            throw new BizException(ErrorCode.ORDER_EXPIRED, "订单已超时，请重新下单");
        }

        // 幂等：已存在未支付的支付单则直接复用，避免用户反复点击生成多张支付单
        PaymentOrder existing = paymentOrderMapper.selectWaitPayByOrderId(order.getOrderId());
        if (existing != null) {
            log.info("复用未支付的支付单: orderId={}, paymentId={}, paymentChannel={}",
                    order.getOrderId(), existing.getPaymentId(), existing.getPaymentChannel());
            return PaymentVO.from(existing);
        }

        PaymentOrder payment = new PaymentOrder();
        payment.setPaymentId(IdGenerator.nextPaymentId());
        payment.setOrderId(order.getOrderId());
        payment.setUid(currentUid);
        // 金额取自订单，不接收客户端传值
        payment.setAmount(order.getTotalAmount());
        payment.setStatus(PaymentStatus.WAIT_PAY.getCode());
        payment.setPaymentChannel(channel.getCode());
        // 支付截止时间与订单保持一致：订单被超时取消时，支付单必然也已过期
        payment.setExpireAt(order.getExpireAt());
        paymentOrderMapper.insert(payment);

        log.info("创建支付单成功: paymentId={}, orderId={}, uid={}, amount={}, paymentChannel={}, expireAt={}",
                payment.getPaymentId(), order.getOrderId(), currentUid, payment.getAmount(),
                channel.getDescription(), payment.getExpireAt());
        return PaymentVO.from(payment);
    }

    @Override
    @Transactional(rollbackFor = Exception.class)
    public void handleCallback(PaymentCallbackRequest request) {
        PaymentChannel channel = PaymentChannel.of(request.paymentChannel());
        // 1. 验签：不通过直接拒绝，且不留痕（来源不可信）
        paymentSigner.verify(request);
        log.info("收到支付回调: paymentId={}, transactionId={}, tradeStatus={}, amount={}",
                request.paymentId(), request.transactionId(), request.tradeStatus(), request.amount());

        PaymentOrder payment = paymentOrderMapper.selectByPaymentId(request.paymentId());
        if (payment == null) {
            throw new BizException(ErrorCode.PAYMENT_NOT_FOUND, "支付单不存在: " + request.paymentId());
        }
        if (!channel.is(payment.getPaymentChannel())) {
            throw new BizException(ErrorCode.PAYMENT_STATUS_CONFLICT,
                    "支付渠道与支付单不一致: 回调=" + request.paymentChannel() + ", 支付单=" + payment.getPaymentChannel());
        }

        // 2. 幂等：渠道未收到应答会重试，已支付的支付单直接确认，让渠道停止重试
        if (PaymentStatus.PAID.is(payment.getStatus())) {
            log.warn("重复支付回调，直接确认: paymentId={}, transactionId={}", request.paymentId(), request.transactionId());
            paymentRecordWriter.append(request, CallbackStatus.SUCCESS);
            return;
        }

        // 3. 渠道明确返回支付失败：留痕并把支付单置为失败，用户可重新发起支付
        if (CallbackStatus.FAILED == request.tradeStatus()) {
            paymentOrderMapper.updateStatus(payment.getPaymentId(), PaymentStatus.FAILED.getCode(),
                    PaymentStatus.WAIT_PAY.getCode());
            paymentRecordWriter.append(request, CallbackStatus.FAILED);
            log.warn("收到支付失败回调: paymentId={}, transactionId={}", request.paymentId(), request.transactionId());
            return;
        }

        // 4. 金额校验：防止渠道金额错传或报文被伪造
        if (!payment.getAmount().equals(request.amount())) {
            paymentRecordWriter.append(request, CallbackStatus.FAILED);
            throw new BizException(ErrorCode.PAYMENT_AMOUNT_MISMATCH,
                    "回调金额与支付单金额不一致: 支付单=" + payment.getAmount() + ", 回调=" + request.amount());
        }
        // 5. 渠道流水号唯一：同一笔渠道交易不能被两张支付单认领
        PaymentOrder bound = paymentOrderMapper.selectByTransactionId(request.transactionId());
        if (bound != null && !bound.getPaymentId().equals(payment.getPaymentId())) {
            paymentRecordWriter.append(request, CallbackStatus.FAILED);
            throw new BizException(ErrorCode.PAYMENT_TRANSACTION_CONFLICT,
                    "渠道流水号已被其它支付单占用: " + request.transactionId());
        }

        LocalDateTime paidAt = LocalDateTime.now();
        if (paymentOrderMapper.markPaid(payment.getPaymentId(), request.transactionId(), paidAt) == 0) {
            // 并发重复回调：另一个线程已完成流转
            log.warn("并发重复回调，本次不再变更支付单: paymentId={}, transactionId={}",
                    request.paymentId(), request.transactionId());
            paymentRecordWriter.append(request, CallbackStatus.SUCCESS);
            return;
        }

        // 6. 驱动订单状态机：WAIT_PAY → PAID，随后由权益发放决定是否 → COMPLETED
        orderService.markPaid(payment.getOrderId(), paidAt);
        paymentRecordWriter.append(request, CallbackStatus.SUCCESS);
        log.info("支付回调处理完成: paymentId={}, orderId={}, transactionId={}",
                request.paymentId(), payment.getOrderId(), request.transactionId());
    }

    private String requireUid(String uid) {
        if (uid == null || uid.isBlank()) {
            throw new BizException(ErrorCode.UNAUTHORIZED, "缺少用户身份信息，请通过网关登录后访问");
        }
        return uid.trim();
    }
}
