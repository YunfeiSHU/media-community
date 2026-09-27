package com.example.authservice.service;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.SecureRandom;
import java.time.Duration;
import java.time.Instant;
import java.util.HashSet;
import java.util.HexFormat;
import java.util.List;
import java.util.Set;
import java.util.UUID;

import org.springframework.data.redis.core.Cursor;
import org.springframework.data.redis.core.HashOperations;
import org.springframework.data.redis.core.ScanOptions;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.script.DefaultRedisScript;
import org.springframework.stereotype.Service;

import com.example.authservice.common.BizException;
import com.example.authservice.common.ErrorCode;

/**
 * Refresh Token 会话（Redis Hash：一个会话一个 key，存 refreshTokenHash 等字段）。
 *
 * <p>注意：{@code Session} 里的 {@code refreshToken} 是给调用方回传给客户端用的明文，
 * Redis 里只存它的 SHA-256（{@code refreshTokenHash}），即使 Redis 被读取也无法直接冒用。
 */
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

    /**
     * 校验 Refresh Token 是否与会话里的哈希一致。
     *
     * <p>该方法已被移除：它把“校验 + 轮换”拆成了两步（典型的 TOCTOU），
     * 并发下同一个 Refresh Token 能被换出两份新会话。现在统一走 {@link #rotate}，
     * 它在一个 Lua 脚本里完成比对与替换，天然是原子的。
     */

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

    /**
     * 删除某个用户的所有会话（强制下线）。
     *
     * <p>用 SCAN 而**不是** KEYS：KEYS 是 O(N) 全库扫描、会阻塞 Redis 主线程，
     * 用户量上来后会拖慢整个实例上的所有命令。
     */
    public void deleteByUid(String uid) {
        if (uid == null || uid.isBlank()) {
            return;
        }
        Set<String> keys = new HashSet<>();
        ScanOptions options = ScanOptions.scanOptions()
                .match(sessionKeyPrefix(uid) + "*")
                .count(200)
                .build();
        try (Cursor<String> cursor = redisTemplate.scan(options)) {
            while (cursor.hasNext()) {
                keys.add(cursor.next());
            }
        }
        if (!keys.isEmpty()) {
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

    private String sessionKeyPrefix(String uid) {
        return "auth:session:" + uid + ":";
    }

    private String sessionKey(String uid, String sessionId) {
        return sessionKeyPrefix(uid) + sessionId;
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
