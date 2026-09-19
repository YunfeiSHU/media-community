package com.example.authservice.verification.provider;

import com.example.authservice.verification.enums.VerificationType;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Component;

@Component
public class SmsNotificationProvider implements NotificationProvider {

    private static final Logger log = LoggerFactory.getLogger(SmsNotificationProvider.class);

    @Override
    public VerificationType type() {
        return VerificationType.PHONE;
    }

    @Override
    public void send(String target, String code) {
        log.info("SMS verification code sent to {}: {}", target, code);
    }
}
