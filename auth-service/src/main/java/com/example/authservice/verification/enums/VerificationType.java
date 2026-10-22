package com.example.authservice.verification.enums;

import com.fasterxml.jackson.annotation.JsonCreator;

public enum VerificationType {
    PHONE,
    EMAIL;

    @JsonCreator
    public static VerificationType fromValue(String value) {
        return valueOf(value.toUpperCase());
    }
}
