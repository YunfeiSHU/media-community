package com.example.order.mapper;

import com.example.order.domain.entity.PaymentRecord;
import java.util.List;
import org.apache.ibatis.annotations.Param;

/**
 * 支付回调流水表 Mapper。SQL 见 {@code resources/mapper/PaymentRecordMapper.xml}。
 */
public interface PaymentRecordMapper {

    /** 追加一条回调流水 */
    int insert(PaymentRecord paymentRecord);

    /** 查询某个支付单的全部回调流水，按时间倒序 */
    List<PaymentRecord> selectByPaymentId(@Param("paymentId") String paymentId);
}
