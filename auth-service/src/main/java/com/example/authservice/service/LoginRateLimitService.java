package com.example.authservice.service;

import java.time.Duration;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import com.example.authservice.common.BizException;
import com.example.authservice.common.ErrorCode;

@Service
public class LoginRateLimitService {

    private static final String KEY_PREFIX = "auth:login:rate:ip:";

    private final StringRedisTemplate redisTemplate;
    private final long maxAttempts;
    private final Duration window;

    public LoginRateLimitService(StringRedisTemplate redisTemplate,
                                 @Value("${auth.login-rate-limit.max-attempts:5}") long maxAttempts,
                                 @Value("${auth.login-rate-limit.window-seconds:60}") long windowSeconds) {
        this.redisTemplate = redisTemplate;
        this.maxAttempts = maxAttempts;
        this.window = Duration.ofSeconds(windowSeconds);
    }

    public void check(String ipAddress) {
        if (ipAddress == null || ipAddress.isBlank()) {
            throw new BizException(ErrorCode.INVALID_PARAM, "无法识别客户端 IP");
        }

        String key = KEY_PREFIX + ipAddress;
        Long count = redisTemplate.opsForValue().increment(key);
        if (count != null && count == 1L) {
            redisTemplate.expire(key, window);
        }
        if (count != null && count > maxAttempts) {
            throw new BizException(ErrorCode.TOO_MANY_REQUESTS, "登录请求过于频繁");
        }
    }
}