package com.example.authservice.verification.provider;

import com.example.authservice.verification.enums.VerificationType;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Component;

@Component
public class EmailNotificationProvider implements NotificationProvider {

    private static final Logger log = LoggerFactory.getLogger(EmailNotificationProvider.class);

    @Override
    public VerificationType type() {
        return VerificationType.EMAIL;
    }

    @Override
    public void send(String target, String code) {
        log.info("Email verification code sent to {}: {}", target, code);
    }
}
