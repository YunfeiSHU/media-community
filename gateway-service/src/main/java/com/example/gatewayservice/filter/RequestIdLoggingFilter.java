package com.example.gatewayservice.filter;

import java.time.Duration;
import java.util.UUID;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.slf4j.MDC;
import org.springframework.cloud.gateway.filter.GatewayFilterChain;
import org.springframework.cloud.gateway.filter.GlobalFilter;
import org.springframework.core.Ordered;
import org.springframework.http.server.reactive.ServerHttpRequest;
import org.springframework.http.server.reactive.ServerHttpResponse;
import org.springframework.stereotype.Component;
import org.springframework.web.server.ServerWebExchange;

import reactor.core.publisher.Mono;

@Component
public class RequestIdLoggingFilter implements GlobalFilter, Ordered {

    private static final Logger log = LoggerFactory.getLogger(RequestIdLoggingFilter.class);

    private static final String REQUEST_ID_HEADER = "X-Request-Id";
    private static final String MDC_REQUEST_ID_KEY = "requestId";
    private static final String MDC_PATH_KEY = "path";
    private static final String MDC_METHOD_KEY = "method";
    private static final String MDC_STATUS_KEY = "status";
    private static final String MDC_LATENCY_KEY = "latency";
    private static final String MDC_ROUTE_KEY = "route";

    @Override
    public Mono<Void> filter(ServerWebExchange exchange, GatewayFilterChain chain) {
        ServerHttpRequest request = exchange.getRequest();
        String requestId = resolveRequestId(request);
        String path = request.getURI().getPath();
        String method = request.getMethod() != null ? request.getMethod().name() : "UNKNOWN";

        ServerHttpRequest mutatedRequest = request.mutate()
                .header(REQUEST_ID_HEADER, requestId)
                .build();

        ServerWebExchange mutatedExchange = exchange.mutate().request(mutatedRequest).build();
        long startNanos = System.nanoTime();

        return chain.filter(mutatedExchange)
                .doFinally(signalType -> {
                    ServerHttpResponse response = mutatedExchange.getResponse();
                    var responseStatus = response.getStatusCode();
                    int status = responseStatus != null ? responseStatus.value() : 200;
                    long latencyMs = Duration.ofNanos(System.nanoTime() - startNanos).toMillis();
                    // gatewayRouteId 由 RoutePredicateHandlerMapping 在 chain.filter() 之后写入，必须在 doFinally 内取值
                    String route = resolveRoute(mutatedExchange);

                    MDC.put(MDC_REQUEST_ID_KEY, requestId);
                    MDC.put(MDC_PATH_KEY, path);
                    MDC.put(MDC_METHOD_KEY, method);
                    MDC.put(MDC_STATUS_KEY, String.valueOf(status));
                    MDC.put(MDC_LATENCY_KEY, latencyMs + "ms");
                    MDC.put(MDC_ROUTE_KEY, route);
                    try {
                        // 交给 logback 输出：与 logback-spring.xml 的 requestId 模式对齐，同时落地到文件
                        log.info("access path={} method={} status={} latency={}ms route={}",
                                path, method, status, latencyMs, route);
                    } finally {
                        MDC.remove(MDC_REQUEST_ID_KEY);
                        MDC.remove(MDC_PATH_KEY);
                        MDC.remove(MDC_METHOD_KEY);
                        MDC.remove(MDC_STATUS_KEY);
                        MDC.remove(MDC_LATENCY_KEY);
                        MDC.remove(MDC_ROUTE_KEY);
                    }
                });
    }

    @Override
    public int getOrder() {
        // 在全局限流之后执行
        return Ordered.HIGHEST_PRECEDENCE + 1;
    }

    private String resolveRequestId(ServerHttpRequest request) {
        String requestId = request.getHeaders().getFirst(REQUEST_ID_HEADER);
        return (requestId == null || requestId.isBlank()) ? UUID.randomUUID().toString() : requestId;
    }

    private String resolveRoute(ServerWebExchange exchange) {
        Object routeId = exchange.getAttribute("org.springframework.cloud.gateway.support.ServerWebExchangeUtils.gatewayRouteId");
        if (routeId != null) {
            return routeId.toString();
        }
        Object uri = exchange.getAttribute("org.springframework.cloud.gateway.support.ServerWebExchangeUtils.GATEWAY_REQUEST_URL_ATTR");
        return uri != null ? uri.toString() : "unknown";
    }
}