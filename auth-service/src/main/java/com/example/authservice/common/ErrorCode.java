package com.example.authservice.common;

import org.springframework.http.HttpStatus;

/**
 * 业务错误码。
 *
 * <p>跨服务统一规范（auth / user / product / order 一致）：
 * <ul>
 *     <li>{@code 40000} 通用业务失败（HTTP 400，凭据错误 / 联系方式已注册 / 验证码无效等）</li>
 *     <li>{@code 40001} 参数不合法（HTTP 400）</li>
 *     <li>{@code 40101} 未认证（HTTP 401）</li>
 *     <li>{@code 40301} 无权限（HTTP 403）</li>
 *     <li>{@code 429xx} 触发限流（HTTP 429，当前只有 42901）</li>
 *     <li>{@code 50000} 服务器内部错误（HTTP 500）</li>
 * </ul>
 *
 * <p>前三位与 HTTP 状态码保持一致，便于网关/前端统一处理。
 */
public enum ErrorCode {

    INVALID_PARAM(40001, "请求参数不合法", HttpStatus.BAD_REQUEST),

    /** 通用业务错误：凭据错误、联系方式已注册、验证码/令牌无效等 */
    BUSINESS_ERROR(40000, "业务处理失败", HttpStatus.BAD_REQUEST),

    UNAUTHORIZED(40101, "未登录或登录已过期", HttpStatus.UNAUTHORIZED),
    FORBIDDEN(40301, "没有操作权限", HttpStatus.FORBIDDEN),

    TOO_MANY_REQUESTS(42901, "请求过于频繁", HttpStatus.TOO_MANY_REQUESTS),

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
