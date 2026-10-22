package com.example.order.common.http;

/**
 * 全链路请求 ID 常量（与网关 / user-service / auth-service / product-service 保持一致）。
 */
public final class RequestIdConstants {

    /** 全链路透传的请求头名 */
    public static final String HEADER_NAME = "X-Request-Id";

    /** logback 日志模式中引用的 MDC key */
    public static final String MDC_KEY = "requestId";

    private RequestIdConstants() {
    }
}
