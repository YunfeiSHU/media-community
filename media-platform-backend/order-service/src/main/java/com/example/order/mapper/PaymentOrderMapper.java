package com.example.order.mapper;

import com.example.order.domain.entity.PaymentOrder;
import java.time.LocalDateTime;
import org.apache.ibatis.annotations.Param;

/**
 * 支付单表 Mapper。SQL 见 {@code resources/mapper/PaymentOrderMapper.xml}。
 */
public interface PaymentOrderMapper {

    /** 新增支付单，回填自增主键 */
    int insert(PaymentOrder paymentOrder);

    PaymentOrder selectByPaymentId(@Param("paymentId") String paymentId);

    /** 查询订单当前未支付的支付单（用于复用，避免重复创建） */
    PaymentOrder selectWaitPayByOrderId(@Param("orderId") String orderId);

    /** 按渠道流水号查询支付单（防同一流水号被重复绑定到不同支付单） */
    PaymentOrder selectByTransactionId(@Param("transactionId") String transactionId);

    /**
     * 状态流转：WAIT_PAY(0) → PAID(1)，同时回填渠道流水号与支付时间。
     *
     * @return 影响行数，0 表示支付单已不是待支付状态（并发重复回调）
     */
    int markPaid(@Param("paymentId") String paymentId,
                 @Param("transactionId") String transactionId,
                 @Param("paidAt") LocalDateTime paidAt);

    /**
     * 状态流转（带乐观校验）。
     *
     * @param expectedStatus 期望的当前状态，为 null 表示不校验
     * @return 影响行数，0 表示状态已被并发修改
     */
    int updateStatus(@Param("paymentId") String paymentId,
                     @Param("status") int status,
                     @Param("expectedStatus") Integer expectedStatus);

    /** 关闭订单下所有未支付的支付单（订单取消 / 超时） */
    int closeWaitPayByOrderId(@Param("orderId") String orderId, @Param("status") int status);
}
