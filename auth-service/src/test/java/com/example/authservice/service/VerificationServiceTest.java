package com.example.authservice.service;

import java.time.Duration;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import static org.mockito.ArgumentMatchers.anyString;
import org.mockito.Mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ValueOperations;

import com.example.authservice.exception.BusinessException;
import com.example.authservice.verification.enums.VerificationPurpose;
import com.example.authservice.verification.enums.VerificationType;
import com.example.authservice.verification.provider.NotificationProvider;
import com.example.authservice.verification.provider.NotificationProviderFactory;

@ExtendWith(MockitoExtension.class)
class VerificationServiceTest {

    @Mock
    private StringRedisTemplate redisTemplate;

    @Mock
    private NotificationProviderFactory providerFactory;

    @Mock
    private NotificationProvider provider;

    @Mock
    private ValueOperations<String, String> valueOperations;

    private VerificationService verificationService;

    @BeforeEach
    void setUp() {
        verificationService = new VerificationService(redisTemplate, providerFactory);
    }

    @Test
    void sendPhoneCodeNormalizesTargetPersistsTtlAndReturnsCode() {
                when(redisTemplate.opsForValue()).thenReturn(valueOperations);
        when(redisTemplate.hasKey(anyString())).thenReturn(false);
        when(providerFactory.get(VerificationType.PHONE)).thenReturn(provider);

        String code = verificationService.send(VerificationPurpose.REGISTER,
                VerificationType.PHONE, " 13800138000 ");

        assertTrue(code.matches("\\d{6}"));
        verify(provider).send("13800138000", code);
        verify(valueOperations).set(
                "auth:verification:register:phone:13800138000", code, Duration.ofMinutes(3));
        verify(valueOperations).set(
                "auth:verification:register:phone:13800138000:cooldown", "1", Duration.ofSeconds(60));
    }

    @Test
    void sendEmailCodeNormalizesTargetButDoesNotReturnCode() {
                when(redisTemplate.opsForValue()).thenReturn(valueOperations);
        when(redisTemplate.hasKey(anyString())).thenReturn(false);
        when(providerFactory.get(VerificationType.EMAIL)).thenReturn(provider);

        String code = verificationService.send(VerificationPurpose.REGISTER,
                VerificationType.EMAIL, " User@Example.COM ");

        assertNull(code);
        ArgumentCaptor<String> targetCaptor = ArgumentCaptor.forClass(String.class);
        ArgumentCaptor<String> codeCaptor = ArgumentCaptor.forClass(String.class);
        verify(provider).send(targetCaptor.capture(), codeCaptor.capture());
        assertEquals("user@example.com", targetCaptor.getValue());
        assertTrue(codeCaptor.getValue().matches("\\d{6}"));
    }

    @Test
    void sendRejectsTargetDuringCooldown() {
        when(redisTemplate.hasKey(anyString())).thenReturn(true);

        BusinessException exception = assertThrows(BusinessException.class,
                () -> verificationService.send(VerificationPurpose.REGISTER,
                        VerificationType.PHONE, "13800138000"));

        assertEquals("验证码获取过于频繁", exception.getMessage());
        verify(providerFactory, never()).get(VerificationType.PHONE);
    }

    @Test
    void sendRejectsInvalidPhoneBeforeProviderCall() {
        BusinessException exception = assertThrows(BusinessException.class,
                () -> verificationService.send(VerificationPurpose.REGISTER,
                        VerificationType.PHONE, "123"));

        assertEquals("phone 格式不正确", exception.getMessage());
        verify(redisTemplate, never()).hasKey(anyString());
    }

    @Test
    void verifyDeletesCodeAfterSuccessfulComparison() {
                when(redisTemplate.opsForValue()).thenReturn(valueOperations);
        when(valueOperations.get("auth:verification:register:email:user@example.com"))
                .thenReturn("123456");

        verificationService.verify(VerificationPurpose.REGISTER, VerificationType.EMAIL,
                " User@Example.COM ", "123456");

        verify(redisTemplate).delete("auth:verification:register:email:user@example.com");
    }

    @Test
    void verifyRejectsWrongOrExpiredCode() {
                when(redisTemplate.opsForValue()).thenReturn(valueOperations);
        when(valueOperations.get("auth:verification:register:phone:13800138000"))
                .thenReturn(null);

        BusinessException exception = assertThrows(BusinessException.class,
                () -> verificationService.verify(VerificationPurpose.REGISTER,
                        VerificationType.PHONE, "13800138000", "123456"));

        assertEquals("验证码无效或已过期", exception.getMessage());
        verify(redisTemplate, never()).delete(anyString());
    }
}
