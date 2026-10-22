package com.example.authservice.dto;

import com.example.authservice.verification.enums.VerificationType;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;

public record LoginRequest(
        @NotNull(message = "type 不能为空")
        VerificationType type,

        @NotBlank(message = "target 不能为空")
        String target,

        @NotBlank(message = "password 不能为空")
        String password
) {
}
