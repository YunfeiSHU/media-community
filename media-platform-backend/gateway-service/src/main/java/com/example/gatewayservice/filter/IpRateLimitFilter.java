package com.example.gatewayservice.filter;

import org.springframework.cloud.gateway.filter.GatewayFilterChain;
import org.springframework.cloud.gateway.filter.GlobalFilter;
import org.springframework.core.Ordered;
import org.springframework.data.redis.core.ReactiveStringRedisTemplate;
import org.springframework.data.redis.core.script.RedisScript;
import org.springframework.http.HttpStatus;
import org.springframework.http.server.reactive.ServerHttpRequest;
import org.springframework.stereotype.Component;
import org.springframework.util.StringUtils;
import org.springframework.web.server.ServerWebExchange;

import com.example.gatewayservice.config.GatewayRateLimitProperties;

import reactor.core.publisher.Mono;

/**
 * IP 限流过滤器
 * 基于客户端 IP 进行限流
 * 
 * 限流参数：
 * - replenishRate: 每秒补充令牌数
 * - burstCapacity: 最大令牌数
 */
@Component
public class IpRateLimitFilter implements GlobalFilter, Ordered {

    private static final String RATE_LIMIT_SCRIPT = """
            local key = KEYS[1]
            local replenish_rate = tonumber(ARGV[1])
            local burst_capacity = tonumber(ARGV[2])
            local current_time = tonumber(ARGV[3])
            
            local current = redis.call('GET', key)
            local bucket = {}
            
            if current == false then
                bucket.tokens = burst_capacity
                bucket.last_refill = current_time
            else
                bucket = cjson.decode(current)
                local time_elapsed = current_time - bucket.last_refill
                local tokens_added = math.floor(time_elapsed * replenish_rate)
                bucket.tokens = math.min(bucket.tokens + tokens_added, burst_capacity)
                bucket.last_refill = current_time
            end
            
            if bucket.tokens >= 1 then
                bucket.tokens = bucket.tokens - 1
                redis.call('SETEX', key, 3600, cjson.encode(bucket))
                return 1
            else
                redis.call('SETEX', key, 3600, cjson.encode(bucket))
                return 0
            end
            """;

    private final ReactiveStringRedisTemplate redisTemplate;
    private final GatewayRateLimitProperties rateLimitProperties;
    private RedisScript<Long> rateLimitScript;

    public IpRateLimitFilter(ReactiveStringRedisTemplate redisTemplate,
                             GatewayRateLimitProperties rateLimitProperties) {
        this.redisTemplate = redisTemplate;
        this.rateLimitProperties = rateLimitProperties;
        this.rateLimitScript = RedisScript.of(RATE_LIMIT_SCRIPT, Long.class);
    }

    @Override
    public Mono<Void> filter(ServerWebExchange exchange, GatewayFilterChain chain) {
        ServerHttpRequest request = exchange.getRequest();
        String clientIp = getClientIp(request);

        String key = rateLimitProperties.getIpLimit().getKeyPrefix() + ":" + clientIp;
        Integer replenishRate = rateLimitProperties.getIpLimit().getReplenishRate();
        Integer burstCapacity = rateLimitProperties.getIpLimit().getBurstCapacity();
        long currentTime = System.currentTimeMillis() / 1000;

        return redisTemplate.execute(
                rateLimitScript,
                java.util.Collections.singletonList(key),
                String.valueOf(replenishRate),
                String.valueOf(burstCapacity),
                String.valueOf(currentTime)
        ).flatMap(result -> {
            if (result != null && result == 1) {
                return chain.filter(exchange);
            } else {
                exchange.getResponse().setStatusCode(HttpStatus.TOO_MANY_REQUESTS);
                return exchange.getResponse().setComplete();
            }
        }).onErrorResume(ex -> {
            // Redis 异常时，放行请求避免服务不可用
            return chain.filter(exchange);
        }).next();
    }

    /**
     * 获取客户端 IP 地址
     * 支持代理情况下的 X-Forwarded-For 头
     */
    private String getClientIp(ServerHttpRequest request) {
        String xForwardedFor = request.getHeaders().getFirst("X-Forwarded-For");
        if (StringUtils.hasText(xForwardedFor)) {
            // X-Forwarded-For 可能包含多个 IP，取第一个
            return xForwardedFor.split(",")[0].trim();
        }

        String xRealIp = request.getHeaders().getFirst("X-Real-IP");
        if (StringUtils.hasText(xRealIp)) {
            return xRealIp;
        }

        if (request.getRemoteAddress() != null) {
            return request.getRemoteAddress().getAddress().getHostAddress();
        }

        return "unknown";
    }

    @Override
    public int getOrder() {
        // 在全局限流之后，认证之前执行
        return Ordered.HIGHEST_PRECEDENCE + 1;
    }
}
