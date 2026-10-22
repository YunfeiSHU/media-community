package com.example.authservice.service;

import java.time.Instant;
import java.util.Date;
import java.util.UUID;

import javax.crypto.SecretKey;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import io.jsonwebtoken.Claims;
import io.jsonwebtoken.Jwts;
import io.jsonwebtoken.security.Keys;

@Service
public class JwtTokenService {

    private static final String ISSUER = "auth-service";
    private static final String AUDIENCE = "api";

    /**
     * 角色所在的 claim 名。
     *
     * <p>必须与网关 {@code gateway.auth.jwt.role-claim}（默认 {@code role}）保持一致；
     * 网关校验 token 后会把该 claim 写入下游请求头 {@code X-User-Role}。
     */
    private static final String ROLE_CLAIM = "role";

    /** 管理员角色名，网关与下游服务均按此值判定 ADMIN */
    private static final String ADMIN_ROLE_NAME = "ADMIN";

    /** 普通用户角色名 */
    private static final String USER_ROLE_NAME = "user";

    private final SecretKey signingKey;

    public JwtTokenService(@Value("${auth.jwt.secret}") String secret) {
        this.signingKey = Keys.hmacShaKeyFor(secret.getBytes(java.nio.charset.StandardCharsets.UTF_8));
    }

    public String generateAccessToken(String uid, long expiresInSeconds) {
        return generateAccessToken(uid, null, expiresInSeconds);
    }

    public String generateAccessToken(String uid, String jti, long expiresInSeconds) {
        return generateAccessToken(uid, jti, null, expiresInSeconds);
    }

    public String generateAccessToken(String uid, String jti, Integer roleCode, long expiresInSeconds) {
        Instant now = Instant.now();
        var builder = Jwts.builder()
                .issuer(ISSUER)
                .subject(uid)
                .audience().add(AUDIENCE).and()
                .id(jti != null ? jti : UUID.randomUUID().toString())
                .issuedAt(Date.from(now))
                .expiration(Date.from(now.plusSeconds(expiresInSeconds)));
        if (roleCode != null) {
            builder.claim(ROLE_CLAIM, roleCode == 0 ? ADMIN_ROLE_NAME : USER_ROLE_NAME);
        }
        return builder.signWith(signingKey).compact();
    }

    public Claims parseAndValidate(String token) {
        return Jwts.parser()
                .verifyWith(signingKey)
                .build()
                .parseSignedClaims(token)
                .getPayload();
    }
}
