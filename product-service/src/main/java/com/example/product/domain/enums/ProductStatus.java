package com.example.product.domain.enums;

import com.example.product.common.BizException;
import com.example.product.common.ErrorCode;

/**
 * 商品状态。
 */
public enum ProductStatus {

    /** 下架（新建商品的初始状态，可编辑、可删除） */
    OFF_SHELF(0),

    /** 上架 */
    ON_SHELF(1),

    /** 已删除（逻辑删除） */
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
        throw new BizException(ErrorCode.INVALID_PARAM, "非法的商品状态: " + code);
    }

    public boolean is(Integer code) {
        return Integer.valueOf(this.code).equals(code);
    }
}
