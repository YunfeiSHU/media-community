package com.example.authservice.service;

import java.nio.charset.StandardCharsets;
import java.time.Instant;

import javax.crypto.SecretKey;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import io.jsonwebtoken.Claims;
import io.jsonwebtoken.Jwts;
import io.jsonwebtoken.security.Keys;

class JwtTokenServiceTest {

    private static final String SECRET = "change-this-secret-to-a-random-value-at-least-32-bytes";

    private JwtTokenService jwtTokenService;
    private SecretKey signingKey;

    @BeforeEach
    void setUp() {
        jwtTokenService = new JwtTokenService(SECRET);
        signingKey = Keys.hmacShaKeyFor(SECRET.getBytes(StandardCharsets.UTF_8));
    }

    @Test
    void generatedTokenContainsSubjectIssuedAtAndExpiration() {
        Instant before = Instant.now();
        String token = jwtTokenService.generateAccessToken("uid-100", 1800);

        Claims claims = Jwts.parser()
                .verifyWith(signingKey)
                .build()
                .parseSignedClaims(token)
                .getPayload();
        Instant after = Instant.now();

        assertEquals("uid-100", claims.getSubject());
        assertNotNull(claims.getIssuedAt());
        assertNotNull(claims.getExpiration());
        assertTrue(claims.getIssuedAt().toInstant().compareTo(before.minusSeconds(1)) >= 0);
        assertTrue(claims.getExpiration().toInstant()
                .isAfter(claims.getIssuedAt().toInstant().plusSeconds(1799)));
        assertTrue(claims.getExpiration().toInstant().isBefore(after.plusSeconds(1801)));
    }
}
