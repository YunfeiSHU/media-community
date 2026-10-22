package com.example.authservice.verification.provider;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import com.example.authservice.verification.enums.VerificationType;

/**
 * 短信验证码下发（<b>桩实现</b>：没有接真实短信通道，只打日志）。
 *
 * <p>日志是否包含验证码本体由 {@code auth.verification.expose-code} 控制（默认 true 仅便于本地联调），
 * 生产环境必须置为 false，避免验证码落进长期保留的日志文件。
 */
@Component
public class SmsNotificationProvider implements NotificationProvider {

    private static final Logger log = LoggerFactory.getLogger(SmsNotificationProvider.class);

    private final boolean exposeCode;

    public SmsNotificationProvider(@Value("${auth.verification.expose-code:true}") boolean exposeCode) {
        this.exposeCode = exposeCode;
    }

    @Override
    public VerificationType type() {
        return VerificationType.PHONE;
    }

    @Override
    public void send(String target, String code) {
        if (exposeCode) {
            log.info("[学习模式] 短信验证码已生成: target={}, code={}（真实环境应改为短信下发，并关闭 expose-code）",
                    target, code);
        }
        else {
            log.info("短信验证码已下发: target={}", target);
        }
    }
}
