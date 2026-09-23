package com.example.authservice.service;

import java.time.Duration;
import java.time.Instant;

import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import com.example.authservice.common.BizException;
import com.example.authservice.common.ErrorCode;

import io.jsonwebtoken.Claims;

@Service
public class JwtBlacklistService {

    private static final String KEY_PREFIX = "auth:blacklist:";

    private final StringRedisTemplate redisTemplate;
    private final JwtTokenService jwtTokenService;

    public JwtBlacklistService(StringRedisTemplate redisTemplate, JwtTokenService jwtTokenService) {
        this.redisTemplate = redisTemplate;
        this.jwtTokenService = jwtTokenService;
    }

    public void blacklist(String accessToken) {
        Claims claims = parseBearerToken(accessToken);
        String jti = claims.getId();
        Instant expiresAt = claims.getExpiration().toInstant();
        long ttlSeconds = Duration.between(Instant.now(), expiresAt).getSeconds();
        if (jti == null || ttlSeconds <= 0) {
            return;
        }
        redisTemplate.opsForValue().set(key(jti), "1", Duration.ofSeconds(ttlSeconds));
    }

    public boolean isBlacklisted(String jti) {
        return jti != null && Boolean.TRUE.equals(redisTemplate.hasKey(key(jti)));
    }

    private Claims parseBearerToken(String authorization) {
        if (authorization == null || !authorization.startsWith("Bearer ")) {
            throw new BizException(ErrorCode.UNAUTHORIZED, "Access Token 无效");
        }
        try {
            return jwtTokenService.parseAndValidate(authorization.substring(7));
        } catch (RuntimeException exception) {
            throw new BizException(ErrorCode.UNAUTHORIZED, "Access Token 无效");
        }
    }

    private String key(String jti) {
        return KEY_PREFIX + jti;
    }
}
