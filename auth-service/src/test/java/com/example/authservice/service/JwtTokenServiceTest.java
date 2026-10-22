package com.example.authservice.service;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.Collection;

import org.junit.jupiter.api.Test;

import io.jsonwebtoken.Claims;
import io.jsonwebtoken.ExpiredJwtException;

class JwtTokenServiceTest {

    private static final String SECRET = "01234567890123456789012345678901";

    @Test
    void generateAccessTokenShouldIncludeCoreClaims() {
        JwtTokenService service = new JwtTokenService(SECRET);

        String token = service.generateAccessToken("uid-100", "jti-100", 0, 1800);
        Claims claims = service.parseAndValidate(token);

        assertEquals("auth-service", claims.getIssuer());
        assertEquals("uid-100", claims.getSubject());
        assertTrue(containsAudience(claims.getAudience(), "api"));
        assertEquals("jti-100", claims.getId());
        assertEquals("ADMIN", claims.get("role"));
        assertNotNull(claims.getIssuedAt());
        assertNotNull(claims.getExpiration());
    }

    @Test
    void generateAccessTokenShouldDefaultJtiWhenMissing() {
        JwtTokenService service = new JwtTokenService(SECRET);

        String token = service.generateAccessToken("uid-100", 1800);
        Claims claims = service.parseAndValidate(token);

        assertEquals("uid-100", claims.getSubject());
        assertNotNull(claims.getId());
        assertTrue(containsAudience(claims.getAudience(), "api"));
    }

    @Test
    void parseAndValidateShouldRejectExpiredToken() {
        JwtTokenService service = new JwtTokenService(SECRET);

        String token = service.generateAccessToken("uid-100", 0);

        assertThrows(ExpiredJwtException.class, () -> service.parseAndValidate(token));
    }

    private static boolean containsAudience(Object audience, String expected) {
        if (audience instanceof Collection<?> collection) {
            return collection.contains(expected);
        }
        return expected.equals(audience);
    }
}