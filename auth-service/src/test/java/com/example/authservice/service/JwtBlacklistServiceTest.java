package com.example.authservice.service;

import java.time.Duration;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ValueOperations;

@ExtendWith(MockitoExtension.class)
class JwtBlacklistServiceTest {

    private static final String SECRET = "change-this-secret-to-a-random-value-at-least-32-bytes";

    @Mock
    private StringRedisTemplate redisTemplate;

    @Mock
    private ValueOperations<String, String> valueOperations;

    private JwtBlacklistService blacklistService;

    @BeforeEach
    void setUp() {
        blacklistService = new JwtBlacklistService(redisTemplate, new JwtTokenService(SECRET));
    }

    @Test
    void blacklistStoresJtiWithRemainingTokenLifetime() {
        when(redisTemplate.opsForValue()).thenReturn(valueOperations);
        String token = new JwtTokenService(SECRET).generateAccessToken("uid-100", "jti-100", 1800);

        blacklistService.blacklist("Bearer " + token);

        ArgumentCaptor<String> keyCaptor = ArgumentCaptor.forClass(String.class);
        ArgumentCaptor<String> ttlCaptor = ArgumentCaptor.forClass(String.class);
        ArgumentCaptor<Duration> durationCaptor = ArgumentCaptor.forClass(Duration.class);
        verify(valueOperations).set(keyCaptor.capture(), ttlCaptor.capture(), durationCaptor.capture());
        assertEquals("auth:blacklist:jti-100", keyCaptor.getValue());
        assertEquals("1", ttlCaptor.getValue());
        assertTrue(durationCaptor.getValue().getSeconds() >= 1797);
        assertTrue(durationCaptor.getValue().getSeconds() <= 1800);
    }

    @Test
    void blacklistRejectsNonBearerToken() {
        assertThrows(RuntimeException.class, () -> blacklistService.blacklist("invalid-token"));
    }

    @Test
    void isBlacklistedReadsJtiKey() {
        when(redisTemplate.hasKey("auth:blacklist:jti-100")).thenReturn(true);

        assertEquals(true, blacklistService.isBlacklisted("jti-100"));
        verify(redisTemplate).hasKey("auth:blacklist:jti-100");
    }
}
