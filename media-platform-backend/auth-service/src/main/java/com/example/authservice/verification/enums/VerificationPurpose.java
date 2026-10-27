package com.example.authservice.verification.enums;

import com.fasterxml.jackson.annotation.JsonCreator;

public enum VerificationPurpose {
    LOGIN,
    REGISTER,
    RESET_PASSWORD;

    @JsonCreator
    public static VerificationPurpose fromValue(String value) {
        return valueOf(value.toUpperCase());
    }
}
