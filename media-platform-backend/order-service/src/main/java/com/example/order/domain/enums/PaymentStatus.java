package com.example.order.domain.enums;

import com.example.order.common.BizException;
import com.example.order.common.ErrorCode;

/**
 * 支付单状态（对应 {@code payment_order.status}）。
 */
public enum PaymentStatus {

    /** 待支付：创建支付单后的初始状态 */
    WAIT_PAY(0, "待支付"),

    /** 已支付：渠道回调成功后进入 */
    PAID(1, "已支付"),

    /** 已关闭：订单被取消/超时，支付单随之关闭 */
    CLOSED(2, "已关闭"),

    /** 支付失败：渠道明确返回失败 */
    FAILED(3, "支付失败");

    private final int code;

    private final String description;

    PaymentStatus(int code, String description) {
        this.code = code;
        this.description = description;
    }

    public int getCode() {
        return code;
    }

    public String getDescription() {
        return description;
    }

    public static PaymentStatus of(Integer code) {
        if (code != null) {
            for (PaymentStatus status : values()) {
                if (status.code == code) {
                    return status;
                }
            }
        }
        throw new BizException(ErrorCode.INVALID_PARAM, "非法的支付单状态: " + code);
    }

    public boolean is(Integer code) {
        return Integer.valueOf(this.code).equals(code);
    }
}
