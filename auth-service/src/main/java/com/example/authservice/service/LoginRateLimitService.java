package com.example.authservice.service;

import java.time.Duration;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import com.example.authservice.exception.BusinessException;

@Service
public class LoginRateLimitService {

    private static final int RATE_LIMIT_ERROR_CODE = 1003;
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
            throw new BusinessException(RATE_LIMIT_ERROR_CODE, "无法识别客户端 IP");
        }

        String key = KEY_PREFIX + ipAddress;
        Long count = redisTemplate.opsForValue().increment(key);
        if (count != null && count == 1L) {
            redisTemplate.expire(key, window);
        }
        if (count != null && count > maxAttempts) {
            throw new BusinessException(RATE_LIMIT_ERROR_CODE, "登录请求过于频繁");
        }
    }
}