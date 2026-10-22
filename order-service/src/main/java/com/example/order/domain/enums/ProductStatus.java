package com.example.order.domain.enums;

import com.example.order.common.BizException;
import com.example.order.common.ErrorCode;

/**
 * 商品状态：<b>消费方镜像</b>（order-service 不依赖 product-service 的代码，只在本地保留一份枚举用于比对）。
 *
 * <p>注意维护风险：两侧靠“人手保持一致”，product-service 侧改了状态码这里不会自动跟着变。
 * 当前取值已与 product-service 的 {@code ProductStatus} 逐一对齐（0/1/2）。
 * 本枚举只用到 {@link #is(Integer)}（直接按码值比对），
 * {@link #of(Integer)} 保留为“码值 → 枚举”的转换入口，非法值会抛 40001（与同包的 OrderStatus / PaymentStatus 一致）。
 */
public enum ProductStatus {

    /** 下架：不可购买 */
    OFF_SHELF(0),

    /** 上架：可购买 */
    ON_SHELF(1),

    /** 已删除 */
    DELETED(2);

    private final int code;

    ProductStatus(int code) {
        this.code = code;
    }

    public int getCode() {
        return code;
    }

    public static ProductStatus of(Integer code) {
        if (code != null) {
            for (ProductStatus status : values()) {
                if (status.code == code) {
                    return status;
                }
            }
        }
        // 早期版本静默返回 OFF_SHELF，会让非法码值看起来像“已下架”；改为显式报错
        throw new BizException(ErrorCode.INVALID_PARAM, "非法的商品状态: " + code);
    }

    public boolean is(Integer code) {
        return Integer.valueOf(this.code).equals(code);
    }
}
