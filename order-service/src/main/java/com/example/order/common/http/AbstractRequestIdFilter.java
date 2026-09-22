package com.example.order.common.http;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.util.UUID;
import org.slf4j.MDC;
import org.springframework.web.filter.OncePerRequestFilter;

/**
 * 把网关透传的 {@code X-Request-Id} 写入 MDC 并回写响应头。
 *
 * <p>网关已经保证该头存在；缺失时（如绕过网关直连）本地生成一个，保证日志始终可串联。
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
        response.setHeader(RequestIdConstants.HEADER_NAME, requestId);
        try {
            filterChain.doFilter(request, response);
        } finally {
            // Tomcat 线程复用，必须清理，避免 requestId 串到下一个请求
            MDC.remove(RequestIdConstants.MDC_KEY);
        }
    }
}
