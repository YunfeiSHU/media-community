package com.example.gatewayservice.service;

import org.springframework.data.redis.core.ReactiveStringRedisTemplate;
import org.springframework.stereotype.Service;

import reactor.core.publisher.Mono;

@Service
public class RedisBlacklistService {

    private static final String KEY_PREFIX = "auth:blacklist:";

    private final ReactiveStringRedisTemplate redisTemplate;

    public RedisBlacklistService(ReactiveStringRedisTemplate redisTemplate) {
        this.redisTemplate = redisTemplate;
    }

    /**
     * 检查 JWT 的 jti 是否在黑名单中
     */
    public Mono<Boolean> isBlacklisted(String jti) {
        if (jti == null || jti.isEmpty()) {
            return Mono.just(false);
        }
        return redisTemplate.hasKey(key(jti));
    }

    private String key(String jti) {
        return KEY_PREFIX + jti;
    }
}
