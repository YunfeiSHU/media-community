package com.example.order.domain.enums;

/**
 * 商品状态：<b>镜像 product-service 的定义</b>（order-service 不依赖其代码，只在消费方保留一份枚举用于校验）。
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
        return OFF_SHELF;
    }

    public boolean is(Integer code) {
        return Integer.valueOf(this.code).equals(code);
    }
}
