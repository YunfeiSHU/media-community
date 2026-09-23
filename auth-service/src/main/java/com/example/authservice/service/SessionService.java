package com.example.authservice.service;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.SecureRandom;
import java.time.Duration;
import java.time.Instant;
import java.util.HexFormat;
import java.util.List;
import java.util.Set;
import java.util.UUID;

import org.springframework.data.redis.core.HashOperations;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.script.DefaultRedisScript;
import org.springframework.stereotype.Service;

import com.example.authservice.common.BizException;
import com.example.authservice.common.ErrorCode;

@Service
public class SessionService {

    private static final Duration REFRESH_TOKEN_TTL = Duration.ofDays(7);
    private static final SecureRandom RANDOM = new SecureRandom();
    private static final DefaultRedisScript<Long> ROTATE_SCRIPT = new DefaultRedisScript<>("""
            local expectedHash = redis.call('HGET', KEYS[1], 'refreshTokenHash')
            if not expectedHash or expectedHash ~= ARGV[1] then
                return 0
            end
            redis.call('DEL', KEYS[1])
            redis.call('HSET', KEYS[2],
                'uid', ARGV[2],
                'jti', ARGV[3],
                'refreshTokenHash', ARGV[4],
                'createdAt', ARGV[5],
                'expiresAt', ARGV[6])
            redis.call('EXPIRE', KEYS[2], ARGV[7])
            return 1
            """, Long.class);

    private final StringRedisTemplate redisTemplate;

    public SessionService(StringRedisTemplate redisTemplate) {
        this.redisTemplate = redisTemplate;
    }

    public Session create(String uid) {
        return create(uid, UUID.randomUUID().toString());
    }

    public Session create(String uid, String jti) {
        String sessionId = UUID.randomUUID().toString().replace("-", "");
        String refreshToken = uid + "." + sessionId + "." + randomSecret();
        Instant now = Instant.now();
        String key = sessionKey(uid, sessionId);
        HashOperations<String, String, String> hash = redisTemplate.opsForHash();
        hash.put(key, "uid", uid);
        hash.put(key, "jti", jti);
        hash.put(key, "refreshTokenHash", sha256(refreshToken));
        hash.put(key, "createdAt", now.toString());
        hash.put(key, "expiresAt", now.plus(REFRESH_TOKEN_TTL).toString());
        redisTemplate.expire(key, REFRESH_TOKEN_TTL);
        return new Session(refreshToken, REFRESH_TOKEN_TTL.getSeconds());
    }

    public void verify(String refreshToken) {
        String[] parts = refreshToken.split("\\.", 3);
        if (parts.length != 3) {
            throw invalidRefreshToken();
        }
        String key = sessionKey(parts[0], parts[1]);
        HashOperations<String, String, String> hash = redisTemplate.opsForHash();
        String expectedHash = hash.get(key, "refreshTokenHash");
        if (expectedHash == null || !MessageDigest.isEqual(
                expectedHash.getBytes(StandardCharsets.UTF_8),
                sha256(refreshToken).getBytes(StandardCharsets.UTF_8))) {
            throw invalidRefreshToken();
        }
    }

    public Session rotate(String refreshToken) {
        return rotate(refreshToken, UUID.randomUUID().toString());
    }

    public Session rotate(String refreshToken, String jti) {
        String[] parts = parse(refreshToken);
        String uid = parts[0];
        String oldSessionKey = sessionKey(uid, parts[1]);
        String newSessionId = UUID.randomUUID().toString().replace("-", "");
        String newRefreshToken = uid + "." + newSessionId + "." + randomSecret();
        Instant now = Instant.now();
        String newSessionKey = sessionKey(uid, newSessionId);
        Long rotated = redisTemplate.execute(
                ROTATE_SCRIPT,
                List.of(oldSessionKey, newSessionKey),
                sha256(refreshToken),
                uid,
                jti,
                sha256(newRefreshToken),
                now.toString(),
                now.plus(REFRESH_TOKEN_TTL).toString(),
                String.valueOf(REFRESH_TOKEN_TTL.getSeconds()));
        if (!Long.valueOf(1L).equals(rotated)) {
            throw invalidRefreshToken();
        }
        return new Session(newRefreshToken, REFRESH_TOKEN_TTL.getSeconds());
    }

    public String uidFrom(String refreshToken) {
        return parse(refreshToken)[0];
    }

    public void deleteByUid(String uid) {
        if (uid == null || uid.isBlank()) {
            return;
        }
        Set<String> keys = redisTemplate.keys("auth:session:" + uid + ":*");
        if (keys != null && !keys.isEmpty()) {
            redisTemplate.delete(keys);
        }
    }

    private String[] parse(String refreshToken) {
        String[] parts = refreshToken.split("\\.", 3);
        if (parts.length != 3 || parts[0].isBlank() || parts[1].isBlank() || parts[2].isBlank()) {
            throw invalidRefreshToken();
        }
        return parts;
    }

    private String sessionKey(String uid, String sessionId) {
        return "auth:session:" + uid + ":" + sessionId;
    }

    private String randomSecret() {
        byte[] bytes = new byte[32];
        RANDOM.nextBytes(bytes);
        return HexFormat.of().formatHex(bytes);
    }

    private String sha256(String value) {
        try {
            return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256")
                    .digest(value.getBytes(StandardCharsets.UTF_8)));
        } catch (NoSuchAlgorithmException exception) {
            throw new IllegalStateException("SHA-256 is unavailable", exception);
        }
    }

    private BizException invalidRefreshToken() {
        return new BizException(ErrorCode.UNAUTHORIZED, "Refresh Token 无效或已过期");
    }

    public record Session(String refreshToken, long expiresIn) {
    }
}
