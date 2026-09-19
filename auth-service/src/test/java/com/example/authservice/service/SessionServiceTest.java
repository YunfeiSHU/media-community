package com.example.authservice.service;

import java.util.Set;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyList;
import org.mockito.Mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.script.DefaultRedisScript;

import com.example.authservice.exception.BusinessException;

@ExtendWith(MockitoExtension.class)
class SessionServiceTest {

    @Mock
    private StringRedisTemplate redisTemplate;

    private SessionService sessionService;

    @BeforeEach
    void setUp() {
        sessionService = new SessionService(redisTemplate);
    }

    @Test
    void rotateReturnsNewRefreshTokenWhenRedisRotationSucceeds() {
        when(redisTemplate.execute(any(DefaultRedisScript.class), anyList(), any(Object[].class)))
                .thenReturn(1L);

        String oldRefreshToken = "user-100.session-old.secret";
        SessionService.Session rotated = sessionService.rotate(oldRefreshToken);

        assertTrue(rotated.refreshToken().startsWith("user-100."));
        assertNotEquals(oldRefreshToken, rotated.refreshToken());
        assertEquals(604800, rotated.expiresIn());
        verify(redisTemplate).execute(any(DefaultRedisScript.class), anyList(), any(Object[].class));
    }

    @Test
    void rotateRejectsRefreshTokenWhenRedisRotationFails() {
        when(redisTemplate.execute(any(DefaultRedisScript.class), anyList(), any(Object[].class)))
                .thenReturn(0L);

        BusinessException exception = assertThrows(BusinessException.class,
                () -> sessionService.rotate("user-100.session-old.secret"));

        assertEquals("Refresh Token 无效或已过期", exception.getMessage());
    }

    @Test
    void rotateRejectsMalformedRefreshTokenBeforeRedisCall() {
        assertThrows(BusinessException.class, () -> sessionService.rotate("malformed-token"));
    }

    @Test
    void deleteByUidDeletesAllRedisSessionKeysForUser() {
        when(redisTemplate.keys("auth:session:user-100:*"))
                .thenReturn(Set.of("auth:session:user-100:s1", "auth:session:user-100:s2"));

        sessionService.deleteByUid("user-100");

        verify(redisTemplate).delete(Set.of("auth:session:user-100:s1", "auth:session:user-100:s2"));
    }
}
