package com.example.order.common;

import org.springframework.http.HttpStatus;

/**
 * 业务错误码。前三位与 HTTP 状态码保持一致，便于网关/前端统一处理。
 */
public enum ErrorCode {

    INVALID_PARAM(40001, "请求参数不合法", HttpStatus.BAD_REQUEST),
    INVALID_PAYMENT_SIGN(40002, "支付回调签名校验失败", HttpStatus.BAD_REQUEST),

    UNAUTHORIZED(40101, "未登录或登录已过期", HttpStatus.UNAUTHORIZED),

    ORDER_NOT_FOUND(40401, "订单不存在", HttpStatus.NOT_FOUND),
    PAYMENT_NOT_FOUND(40402, "支付单不存在", HttpStatus.NOT_FOUND),
    PRODUCT_NOT_FOUND(40403, "商品不存在", HttpStatus.NOT_FOUND),
    SKU_NOT_FOUND(40404, "SKU不存在", HttpStatus.NOT_FOUND),

    ORDER_STATUS_CONFLICT(40901, "订单当前状态不允许该操作", HttpStatus.CONFLICT),
    PRODUCT_STATUS_CONFLICT(40902, "商品当前状态不可购买", HttpStatus.CONFLICT),
    SKU_STATUS_CONFLICT(40903, "SKU当前状态不可购买", HttpStatus.CONFLICT),
    STOCK_NOT_ENOUGH(40904, "可用库存不足", HttpStatus.CONFLICT),
    ORDER_EXPIRED(40905, "订单已超时", HttpStatus.CONFLICT),
    PAYMENT_AMOUNT_MISMATCH(40906, "支付金额与订单金额不一致", HttpStatus.CONFLICT),
    PAYMENT_STATUS_CONFLICT(40907, "支付单当前状态不允许该操作", HttpStatus.CONFLICT),
    PAYMENT_TRANSACTION_CONFLICT(40908, "支付渠道流水号已被占用", HttpStatus.CONFLICT),
    DUPLICATE_REQUEST(40909, "数据已存在，请勿重复提交", HttpStatus.CONFLICT),

    INTERNAL_ERROR(50000, "服务器内部错误", HttpStatus.INTERNAL_SERVER_ERROR),
    RPC_INVOKE_FAILED(50001, "远程服务调用失败，请稍后重试", HttpStatus.SERVICE_UNAVAILABLE);

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
