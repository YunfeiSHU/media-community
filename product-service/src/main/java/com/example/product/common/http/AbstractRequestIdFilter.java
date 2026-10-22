package com.example.product.common.http;

import java.io.IOException;
import java.util.UUID;

import org.slf4j.MDC;
import org.springframework.web.filter.OncePerRequestFilter;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;

/**
 * 把 {@code X-Request-Id} 写入 MDC 的抽象过滤器。
 *
 * <p>网关已经为每个请求生成 / 透传了 {@code X-Request-Id}，这里直接复用；
 * 若直接访问服务（绕过网关）没有该请求头，则自行生成一个，保证日志始终可追踪。
 */
public abstract class AbstractRequestIdFilter extends OncePerRequestFilter {

    @Override
    protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain filterChain)
            throws ServletException, IOException {
        String requestId = request.getHeader(RequestIdConstants.HEADER_NAME);
        if (requestId == null || requestId.isBlank()) {
            requestId = UUID.randomUUID().toString();
        }

        MDC.put(RequestIdConstants.MDC_KEY, requestId);
        // 回写响应头，方便前端 / 网关侧把一次调用与后端日志关联
        response.setHeader(RequestIdConstants.HEADER_NAME, requestId);
        try {
            filterChain.doFilter(request, response);
        } finally {
            // 线程池复用，必须清理，避免 requestId 串到下一个请求
            MDC.remove(RequestIdConstants.MDC_KEY);
        }
    }
}
