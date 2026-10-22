package com.example.order.domain.enums;

import com.example.order.common.BizException;
import com.example.order.common.ErrorCode;

/**
 * SKU 状态：<b>消费方镜像</b>（order-service 不依赖 product-service 的代码，只在本地保留一份枚举用于比对）。
 *
 * <p>注意维护风险：两侧靠“人手保持一致”，product-service 侧改了状态码这里不会自动跟着变。
 * 当前取值已与 product-service 的 {@code SkuStatus} 逐一对齐（0/1/2）。
 * 本枚举只用到 {@link #is(Integer)}（直接按码值比对），
 * {@link #of(Integer)} 保留为“码值 → 枚举”的转换入口，非法值会抛 40001（与同包的 OrderStatus / PaymentStatus 一致）。
 */
public enum SkuStatus {

    /** 禁用：不可售 */
    DISABLED(0),

    /** 启用：可售 */
    ENABLED(1),

    /** 已删除 */
    DELETED(2);

    private final int code;

    SkuStatus(int code) {
        this.code = code;
    }

    public int getCode() {
        return code;
    }

    public static SkuStatus of(Integer code) {
        if (code != null) {
            for (SkuStatus status : values()) {
                if (status.code == code) {
                    return status;
                }
            }
        }
        // 早期版本静默返回 DISABLED，会让非法码值看起来像“已禁用”；改为显式报错
        throw new BizException(ErrorCode.INVALID_PARAM, "非法的SKU状态: " + code);
    }

    public boolean is(Integer code) {
        return Integer.valueOf(this.code).equals(code);
    }
}
