package com.example.gatewayservice.filter;

import org.springframework.cloud.gateway.filter.GatewayFilterChain;
import org.springframework.cloud.gateway.filter.GlobalFilter;
import org.springframework.core.Ordered;
import org.springframework.data.redis.core.ReactiveStringRedisTemplate;
import org.springframework.data.redis.core.script.RedisScript;
import org.springframework.http.HttpStatus;
import org.springframework.stereotype.Component;
import org.springframework.web.server.ServerWebExchange;

import com.example.gatewayservice.config.GatewayRateLimitProperties;

import reactor.core.publisher.Mono;

/**
 * 全局限流过滤器
 * 基于 Redis Lua 脚本实现令牌桶算法
 * 
 * 限流参数：
 * - replenishRate: 每秒补充令牌数
 * - burstCapacity: 最大令牌数
 */
@Component
public class GlobalRateLimitFilter implements GlobalFilter, Ordered {

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

    public GlobalRateLimitFilter(ReactiveStringRedisTemplate redisTemplate,
                                 GatewayRateLimitProperties rateLimitProperties) {
        this.redisTemplate = redisTemplate;
        this.rateLimitProperties = rateLimitProperties;
        this.rateLimitScript = RedisScript.of(RATE_LIMIT_SCRIPT, Long.class);
    }

    @Override
    public Mono<Void> filter(ServerWebExchange exchange, GatewayFilterChain chain) {
        String key = rateLimitProperties.getGlobalLimit().getKeyPrefix();
        Integer replenishRate = rateLimitProperties.getGlobalLimit().getReplenishRate();
        Integer burstCapacity = rateLimitProperties.getGlobalLimit().getBurstCapacity();
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

    @Override
    public int getOrder() {
        // 在所有其他过滤器之前执行
        return Ordered.HIGHEST_PRECEDENCE;
    }
}
