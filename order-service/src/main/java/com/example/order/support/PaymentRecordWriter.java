package com.example.order.support;

import com.example.order.domain.entity.PaymentRecord;
import com.example.order.domain.enums.CallbackStatus;
import com.example.order.dto.request.PaymentCallbackRequest;
import com.example.order.mapper.PaymentRecordMapper;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;

/**
 * 支付回调流水写入器。
 *
 * <p>用独立事务（{@link Propagation#REQUIRES_NEW}）提交：回调处理过程中出现的验签失败、
 * 金额不符等异常会让外层事务回滚，但「渠道确实调过来一次」这件事必须留下痕迹，便于对账与追查。
 */
@Slf4j
@Component
@RequiredArgsConstructor
public class PaymentRecordWriter {

    private final PaymentRecordMapper paymentRecordMapper;

    private final ObjectMapper objectMapper;

    /**
     * 追加一条回调流水。
     *
     * @param status 本次回调的处理结果
     */
    @Transactional(propagation = Propagation.REQUIRES_NEW, rollbackFor = Exception.class)
    public void append(PaymentCallbackRequest request, CallbackStatus status) {
        PaymentRecord record = new PaymentRecord();
        record.setPaymentId(request.paymentId());
        record.setTransactionId(request.transactionId());
        record.setPaymentChannel(request.paymentChannel());
        record.setCallbackStatus(status.getCode());
        record.setCallbackData(toJson(request));
        paymentRecordMapper.insert(record);
        log.debug("记录支付回调流水: paymentId={}, transactionId={}, callbackStatus={}",
                request.paymentId(), request.transactionId(), status.getCode());
    }

    private String toJson(PaymentCallbackRequest request) {
        try {
            return objectMapper.writeValueAsString(request);
        } catch (JsonProcessingException ex) {
            log.warn("支付回调报文序列化失败，退化为 toString: {}", ex.getMessage());
            return request.toString();
        }
    }
}
