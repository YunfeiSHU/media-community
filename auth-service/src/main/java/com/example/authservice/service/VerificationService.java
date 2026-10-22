package com.example.authservice.service;

import java.time.Duration;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import com.example.authservice.common.BizException;
import com.example.authservice.common.ErrorCode;
import com.example.authservice.verification.enums.VerificationPurpose;
import com.example.authservice.verification.enums.VerificationType;
import com.example.authservice.verification.generator.CodeGenerator;
import com.example.authservice.verification.provider.NotificationProviderFactory;

/**
 * 验证码的生成 / 下发 / 校验。
 *
 * <p>验证码只存在 Redis（TTL 3 分钟）与通知通道里；
 * 是否把验证码回传给调用方由 {@code auth.verification.expose-code} 控制（默认 true，仅供本地联调）。
 */
@Service
public class VerificationService {

    private static final Duration CODE_TTL = Duration.ofMinutes(3);
    private static final Duration COOLDOWN = Duration.ofSeconds(60);

    private final StringRedisTemplate redisTemplate;
    private final NotificationProviderFactory providerFactory;
    private final CodeGenerator codeGenerator = new CodeGenerator();

    /**
     * 学习模式：为 true 时 {@link #send} 会返回验证码，接口响应的 {@code data.code} 非空。
     *
     * <p>本项目的 Email/SmsNotificationProvider 只是打日志的桩实现，没有真实通道，
     * 因此默认开启以便本地跑通注册流程；生产环境必须置为 false。
     */
    private final boolean exposeCode;

    public VerificationService(StringRedisTemplate redisTemplate,
                               NotificationProviderFactory providerFactory,
                               @Value("${auth.verification.expose-code:true}") boolean exposeCode) {
        this.redisTemplate = redisTemplate;
        this.providerFactory = providerFactory;
        this.exposeCode = exposeCode;
    }

    /**
     * 生成并下发验证码。
     *
     * @return 学习模式下返回验证码本体；生产模式（expose-code=false）返回 {@code null}
     * @throws BizException 60 秒内重复请求时为 429
     */
    public String send(VerificationPurpose purpose, VerificationType type, String target) {
        String normalizedTarget = normalize(type, target);
        String cooldownKey = cooldownKey(purpose, type, normalizedTarget);
        if (Boolean.TRUE.equals(redisTemplate.hasKey(cooldownKey))) {
            throw new BizException(ErrorCode.TOO_MANY_REQUESTS, "验证码获取过于频繁");
        }

        String code = codeGenerator.generate();
        providerFactory.get(type).send(normalizedTarget, code);
        String codeKey = codeKey(purpose, type, normalizedTarget);
        redisTemplate.opsForValue().set(codeKey, code, CODE_TTL);
        redisTemplate.opsForValue().set(cooldownKey, "1", COOLDOWN);
        return exposeCode ? code : null;
    }

    public void verify(VerificationPurpose purpose, VerificationType type, String target, String code) {
        String normalizedTarget = normalize(type, target);
        String key = codeKey(purpose, type, normalizedTarget);
        String expectedCode = redisTemplate.opsForValue().get(key);
        if (expectedCode == null || !expectedCode.equals(code)) {
            throw new BizException(ErrorCode.BUSINESS_ERROR, "验证码无效或已过期");
        }
        redisTemplate.delete(key);
    }

    private String normalize(VerificationType type, String target) {
        String normalizedTarget = target.trim();
        if (type == VerificationType.EMAIL) {
            if (!normalizedTarget.matches("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$")) {
                throw new BizException(ErrorCode.INVALID_PARAM, "email 格式不正确");
            }
            return normalizedTarget.toLowerCase();
        }
        if (!normalizedTarget.matches("^1[3-9]\\d{9}$")) {
            throw new BizException(ErrorCode.INVALID_PARAM, "phone 格式不正确");
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
