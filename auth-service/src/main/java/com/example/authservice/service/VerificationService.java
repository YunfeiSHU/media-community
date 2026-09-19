package com.example.authservice.service;

import java.time.Duration;

import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import com.example.authservice.exception.BusinessException;
import com.example.authservice.verification.enums.VerificationPurpose;
import com.example.authservice.verification.enums.VerificationType;
import com.example.authservice.verification.generator.CodeGenerator;
import com.example.authservice.verification.provider.NotificationProviderFactory;

@Service
public class VerificationService {

    private static final int BUSINESS_ERROR_CODE = 1001;
    private static final Duration CODE_TTL = Duration.ofMinutes(3);
    private static final Duration COOLDOWN = Duration.ofSeconds(60);

    private final StringRedisTemplate redisTemplate;
    private final NotificationProviderFactory providerFactory;
    private final CodeGenerator codeGenerator = new CodeGenerator();

    public VerificationService(StringRedisTemplate redisTemplate,
                               NotificationProviderFactory providerFactory) {
        this.redisTemplate = redisTemplate;
        this.providerFactory = providerFactory;
    }

    public String send(VerificationPurpose purpose, VerificationType type, String target) {
        String normalizedTarget = normalize(type, target);
        String cooldownKey = cooldownKey(purpose, type, normalizedTarget);
        if (Boolean.TRUE.equals(redisTemplate.hasKey(cooldownKey))) {
            throw new BusinessException(BUSINESS_ERROR_CODE, "验证码获取过于频繁");
        }

        String code = codeGenerator.generate();
        providerFactory.get(type).send(normalizedTarget, code);
        String codeKey = codeKey(purpose, type, normalizedTarget);
        redisTemplate.opsForValue().set(codeKey, code, CODE_TTL);
        redisTemplate.opsForValue().set(cooldownKey, "1", COOLDOWN);
        return type == VerificationType.PHONE ? code : null;
    }

    public void verify(VerificationPurpose purpose, VerificationType type, String target, String code) {
        String normalizedTarget = normalize(type, target);
        String key = codeKey(purpose, type, normalizedTarget);
        String expectedCode = redisTemplate.opsForValue().get(key);
        if (expectedCode == null || !expectedCode.equals(code)) {
            throw new BusinessException(BUSINESS_ERROR_CODE, "验证码无效或已过期");
        }
        redisTemplate.delete(key);
    }

    private String normalize(VerificationType type, String target) {
        String normalizedTarget = target.trim();
        if (type == VerificationType.EMAIL) {
            if (!normalizedTarget.matches("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$")) {
                throw new BusinessException(BUSINESS_ERROR_CODE, "email 格式不正确");
            }
            return normalizedTarget.toLowerCase();
        }
        if (!normalizedTarget.matches("^1[3-9]\\d{9}$")) {
            throw new BusinessException(BUSINESS_ERROR_CODE, "phone 格式不正确");
        }
        return normalizedTarget;
    }

    private String codeKey(VerificationPurpose purpose, VerificationType type, String target) {
        return "auth:verification:%s:%s:%s".formatted(purpose.name().toLowerCase(), type.name().toLowerCase(), target);
    }

    private String cooldownKey(VerificationPurpose purpose, VerificationType type, String target) {
        return codeKey(purpose, type, target) + ":cooldown";
    }
}
