package com.example.order.domain.enums;

import com.example.order.common.BizException;
import com.example.order.common.ErrorCode;

/**
 * 支付渠道（对应 {@code payment_order.payment_channel} / {@code payment_record.payment_channel}）。
 *
 * <p>渠道接入被抽象为「创建支付单 → 得到渠道流水号 → 接收异步回调」，本地演示不接真实渠道。
 */
public enum PaymentChannel {

    /** 微信支付 */
    WECHAT(1, "微信支付"),

    /** 支付宝 */
    ALIPAY(2, "支付宝"),

    /** 账户余额 */
    BALANCE(3, "余额支付");

    private final int code;

    private final String description;

    PaymentChannel(int code, String description) {
        this.code = code;
        this.description = description;
    }

    public int getCode() {
        return code;
    }

    public String getDescription() {
        return description;
    }

    public static PaymentChannel of(Integer code) {
        if (code != null) {
            for (PaymentChannel channel : values()) {
                if (channel.code == code) {
                    return channel;
                }
            }
        }
        throw new BizException(ErrorCode.INVALID_PARAM, "非法的支付渠道: " + code);
    }

    public boolean is(Integer code) {
        return Integer.valueOf(this.code).equals(code);
    }
}
