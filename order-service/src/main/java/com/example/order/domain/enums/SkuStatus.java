package com.example.order.domain.enums;

/**
 * SKU 状态：<b>镜像 product-service 的定义</b>（order-service 不依赖其代码，只在消费方保留一份枚举用于校验）。
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
        return DISABLED;
    }

    public boolean is(Integer code) {
        return Integer.valueOf(this.code).equals(code);
    }
}
