package com.example.authservice.service;

import java.time.Duration;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import com.example.authservice.common.BizException;
import com.example.authservice.common.ErrorCode;

/**
 * 登录限流（按客户端 IP 分桶）。
 *
 * <p>只统计<b>失败尝试</b>：登录成功会调用 {@link #onLoginSuccess(String)} 清零计数。
 * 若把成功登录也算进去，正常用户在一分钟内连登几次就会被误当成暴力破解。
 *
 * <p>IP 取 {@code HttpServletRequest#getRemoteAddr()}。经网关访问时它是网关地址，
 * 因此必须同时开启 {@code server.forward-headers-strategy=framework}（见 application.yml），
 * 让 Spring 先按 {@code X-Forwarded-For} 还原真实客户端地址；
 * 否则所有客户端会共用一个限流桶，任何人刷 5 次就能让全站一分钟内无法登录。
 */
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

    /**
     * 登录前调用：计数 +1，超过阈值抛 429。
     *
     * @throws BizException 无法识别 IP（40001）或超过阈值（42901）
     */
    public void checkBeforeLogin(String ipAddress) {
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

    /** 登录成功后调用：清掉该 IP 的失败计数 */
    public void onLoginSuccess(String ipAddress) {
        if (ipAddress == null || ipAddress.isBlank()) {
            return;
        }
        redisTemplate.delete(KEY_PREFIX + ipAddress);
    }
}