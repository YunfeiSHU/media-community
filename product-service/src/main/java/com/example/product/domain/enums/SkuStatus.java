package com.example.product.domain.enums;

import com.example.product.common.BizException;
import com.example.product.common.ErrorCode;

/**
 * SKU 状态。
 */
public enum SkuStatus {

    /** 禁用（不参与售卖） */
    DISABLED(0),

    /** 启用 */
    ENABLED(1),

    /** 已删除（逻辑删除） */
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
        throw new BizException(ErrorCode.INVALID_PARAM, "非法的SKU状态: " + code);
    }

    public boolean is(Integer code) {
        return Integer.valueOf(this.code).equals(code);
    }
}
