package com.example.authservice.verification.provider;

import com.example.authservice.verification.enums.VerificationType;

public interface NotificationProvider {

    VerificationType type();

    void send(String target, String code);
}
