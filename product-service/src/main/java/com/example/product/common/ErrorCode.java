package com.example.product.common;

import org.springframework.http.HttpStatus;

/**
 * 业务错误码。前三位与 HTTP 状态码保持一致，便于网关/前端统一处理。
 */
public enum ErrorCode {

    INVALID_PARAM(40001, "请求参数不合法", HttpStatus.BAD_REQUEST),

    PRODUCT_NOT_FOUND(40401, "商品不存在", HttpStatus.NOT_FOUND),
    SKU_NOT_FOUND(40402, "SKU不存在", HttpStatus.NOT_FOUND),
    INVENTORY_NOT_FOUND(40403, "库存记录不存在", HttpStatus.NOT_FOUND),

    PRODUCT_STATUS_CONFLICT(40901, "商品当前状态不允许该操作", HttpStatus.CONFLICT),
    SKU_STATUS_CONFLICT(40902, "SKU当前状态不允许该操作", HttpStatus.CONFLICT),
    STOCK_NOT_ENOUGH(40903, "可用库存不足", HttpStatus.CONFLICT),
    STOCK_LOCK_NOT_ENOUGH(40904, "锁定库存不足", HttpStatus.CONFLICT),

    INTERNAL_ERROR(50000, "服务器内部错误", HttpStatus.INTERNAL_SERVER_ERROR);

    private final int code;

    private final String message;

    private final HttpStatus httpStatus;

    ErrorCode(int code, String message, HttpStatus httpStatus) {
        this.code = code;
        this.message = message;
        this.httpStatus = httpStatus;
    }

    public int getCode() {
        return code;
    }

    public String getMessage() {
        return message;
    }

    public HttpStatus getHttpStatus() {
        return httpStatus;
    }
}
