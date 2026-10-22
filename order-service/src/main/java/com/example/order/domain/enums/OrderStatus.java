package com.example.order.domain.enums;

import com.example.order.common.BizException;
import com.example.order.common.ErrorCode;

/**
 * 订单状态机（对应 {@code order.status}）。
 *
 * <pre>
 * WAIT_PAY --支付成功--> PAID --权益发放完成--> COMPLETED
 * WAIT_PAY --用户取消/超时--> CANCELLED
 * </pre>
 */
public enum OrderStatus {

    /** 待支付：创建订单后的初始状态，可取消、可发起支付 */
    WAIT_PAY(0, "待支付"),

    /** 已支付：支付回调成功后进入，等待权益发放 */
    PAID(1, "已支付"),

    /** 已完成：权益发放（库存确认）完成 */
    COMPLETED(2, "已完成"),

    /** 已取消：用户主动取消或超时自动取消 */
    CANCELLED(3, "已取消");

    private final int code;

    private final String description;

    OrderStatus(int code, String description) {
        this.code = code;
        this.description = description;
    }

    public int getCode() {
        return code;
    }

    public String getDescription() {
        return description;
    }

    public static OrderStatus of(Integer code) {
        if (code != null) {
            for (OrderStatus status : values()) {
                if (status.code == code) {
                    return status;
                }
            }
        }
        throw new BizException(ErrorCode.INVALID_PARAM, "非法的订单状态: " + code);
    }

    public boolean is(Integer code) {
        return Integer.valueOf(this.code).equals(code);
    }
}
