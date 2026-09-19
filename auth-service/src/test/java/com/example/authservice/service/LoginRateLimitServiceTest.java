package com.example.authservice.service;

import java.time.Duration;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ValueOperations;

import com.example.authservice.exception.BusinessException;

@ExtendWith(MockitoExtension.class)
class LoginRateLimitServiceTest {

    @Mock
    private StringRedisTemplate redisTemplate;

    @Mock
    private ValueOperations<String, String> valueOperations;

    private LoginRateLimitService rateLimitService;

    @BeforeEach
    void setUp() {
        rateLimitService = new LoginRateLimitService(redisTemplate, 5, 60);
        when(redisTemplate.opsForValue()).thenReturn(valueOperations);
    }

    @Test
    void firstRequestIncrementsCounterAndSetsWindowTtl() {
        when(valueOperations.increment("auth:login:rate:ip:192.168.1.10")).thenReturn(1L);

        rateLimitService.check("192.168.1.10");

        verify(valueOperations).increment("auth:login:rate:ip:192.168.1.10");
        verify(redisTemplate).expire("auth:login:rate:ip:192.168.1.10", Duration.ofSeconds(60));
    }

    @Test
    void requestsWithinLimitDoNotResetExistingWindow() {
        when(valueOperations.increment("auth:login:rate:ip:192.168.1.10")).thenReturn(5L);

        rateLimitService.check("192.168.1.10");

        verify(redisTemplate, never()).expire("auth:login:rate:ip:192.168.1.10", Duration.ofSeconds(60));
    }

    @Test
    void requestOverLimitIsRejected() {
        when(valueOperations.increment("auth:login:rate:ip:192.168.1.10")).thenReturn(6L);

        BusinessException exception = assertThrows(BusinessException.class,
                () -> rateLimitService.check("192.168.1.10"));

        assertEquals(1003, exception.getCode());
        assertEquals("登录请求过于频繁", exception.getMessage());
    }
}