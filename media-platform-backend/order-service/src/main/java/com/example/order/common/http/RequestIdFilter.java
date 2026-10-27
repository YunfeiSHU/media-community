package com.example.order.common.http;

import org.springframework.core.Ordered;
import org.springframework.core.annotation.Order;
import org.springframework.stereotype.Component;

/**
 * 请求 ID 过滤器：最早执行，保证后续所有日志都带 requestId。
 */
@Component
@Order(Ordered.HIGHEST_PRECEDENCE)
public class RequestIdFilter extends AbstractRequestIdFilter {
}
