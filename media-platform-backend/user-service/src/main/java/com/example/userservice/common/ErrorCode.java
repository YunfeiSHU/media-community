package com.example.userservice.common;

import org.springframework.http.HttpStatus;

/**
 * 业务错误码。
 *
 * <p>跨服务统一规范（auth / user / product / order 一致）：
 * <ul>
 *     <li>{@code 40001} 参数不合法（HTTP 400）</li>
 *     <li>{@code 40101} 未认证（HTTP 401）</li>
 *     <li>{@code 40301} 无权限（HTTP 403）</li>
 *     <li>{@code 404xx} 资源不存在（HTTP 404）</li>
 *     <li>{@code 409xx} 状态/唯一性冲突（HTTP 409）</li>
 *     <li>{@code 50000} 服务器内部错误（HTTP 500）</li>
 * </ul>
 *
 * <p>前三位与 HTTP 状态码保持一致，便于网关/前端统一处理。
 */
public enum ErrorCode {

    INVALID_PARAM(40001, "请求参数不合法", HttpStatus.BAD_REQUEST),

    UNAUTHORIZED(40101, "未登录或登录已过期", HttpStatus.UNAUTHORIZED),
    FORBIDDEN(40301, "没有操作权限", HttpStatus.FORBIDDEN),

    USER_NOT_FOUND(40401, "用户不存在", HttpStatus.NOT_FOUND),

    /** 唯一键冲突：本服务的唯一键只有 uid（随机串），所以不会是“用户名已存在” */
    USER_CONFLICT(40901, "用户标识冲突", HttpStatus.CONFLICT),

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
