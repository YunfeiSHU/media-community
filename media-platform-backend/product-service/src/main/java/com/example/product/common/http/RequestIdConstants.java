package com.example.product.common.http;

/**
 * 链路追踪相关的常量。
 *
 * <p>与网关 {@code RequestIdLoggingFilter}、user-service / auth-service 保持完全一致：
 * 请求头统一为 {@code X-Request-Id}，MDC key 统一为 {@code requestId}。
 */
public final class RequestIdConstants {

    /** 全链路透传的请求头名 */
    public static final String HEADER_NAME = "X-Request-Id";

    /** logback 日志模式中引用的 MDC key */
    public static final String MDC_KEY = "requestId";

    private RequestIdConstants() {
    }
}
