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
            builder.claim("role", roleCode == 0 ? "ADMIN" : "user");
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
