package com.example.product.domain.enums;

import com.example.product.common.BizException;
import com.example.product.common.ErrorCode;

/**
 * 商品类型。
 */
public enum ProductType {

    /** 实物商品 */
    PHYSICAL(1),

    /** 虚拟商品 */
    VIRTUAL(2);

    private final int code;

    ProductType(int code) {
        this.code = code;
    }

    public int getCode() {
        return code;
    }

    public static ProductType of(Integer code) {
        if (code != null) {
            for (ProductType type : values()) {
                if (type.code == code) {
                    return type;
                }
            }
        }
        throw new BizException(ErrorCode.INVALID_PARAM,
                "非法的商品类型: " + code + "（1-实物 2-虚拟）");
    }
}
