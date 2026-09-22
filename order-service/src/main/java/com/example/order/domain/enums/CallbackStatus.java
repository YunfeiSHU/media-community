package com.example.order.domain.enums;

import com.example.order.common.BizException;
import com.example.order.common.ErrorCode;

/**
 * 支付回调结果（对应 {@code payment_record.callback_status}）。
 */
public enum CallbackStatus {

    /** 回调成功（渠道侧支付成功） */
    SUCCESS(1, "成功"),

    /** 回调失败（验签通过但渠道侧支付失败 / 金额不符等） */
    FAILED(2, "失败");

    private final int code;

    private final String description;

    CallbackStatus(int code, String description) {
        this.code = code;
        this.description = description;
    }

    public int getCode() {
        return code;
    }

    public String getDescription() {
        return description;
    }

    public static CallbackStatus of(Integer code) {
        if (code != null) {
            for (CallbackStatus status : values()) {
                if (status.code == code) {
                    return status;
                }
            }
        }
        throw new BizException(ErrorCode.INVALID_PARAM, "非法的回调结果: " + code);
    }

    public boolean is(Integer code) {
        return Integer.valueOf(this.code).equals(code);
    }
}
