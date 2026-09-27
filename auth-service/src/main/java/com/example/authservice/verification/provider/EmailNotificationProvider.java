package com.example.authservice.verification.provider;

import com.example.authservice.verification.enums.VerificationType;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

/**
 * 邮件验证码下发（<b>桩实现</b>：没有接真实邮件通道，只打日志）。
 *
 * <p>因为验证码等价于一次性凭据，是否把它写进日志由 {@code auth.verification.expose-code} 控制
 * （默认 true 仅便于本地联调）；生产环境必须置为 false，否则验证码会落进
 * {@code logs/auth-service/auth-service.log}（保留 30 天），任何能读日志的人都能在三分钟内冒用。
 */
@Component
public class EmailNotificationProvider implements NotificationProvider {

    private static final Logger log = LoggerFactory.getLogger(EmailNotificationProvider.class);

    private final boolean exposeCode;

    public EmailNotificationProvider(@Value("${auth.verification.expose-code:true}") boolean exposeCode) {
        this.exposeCode = exposeCode;
    }

    @Override
    public VerificationType type() {
        return VerificationType.EMAIL;
    }

    @Override
    public void send(String target, String code) {
        if (exposeCode) {
            log.info("[学习模式] 邮件验证码已生成: target={}, code={}（真实环境应改为邮件下发，并关闭 expose-code）",
                    target, code);
        }
        else {
            log.info("邮件验证码已下发: target={}", target);
        }
    }
}
