package com.example.product.common.http;

import org.springframework.core.Ordered;
import org.springframework.core.annotation.Order;
import org.springframework.stereotype.Component;

/**
 * 链路 ID 过滤器：注册在最外层，保证后续所有 Filter / Controller / Service 的日志都带上 requestId。
 */
@Component
@Order(Ordered.HIGHEST_PRECEDENCE)
public class RequestIdFilter extends AbstractRequestIdFilter {
}
