package com.example.authservice.dto;

import com.example.authservice.verification.enums.VerificationType;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import com.example.authservice.verification.enums.VerificationPurpose;

public record SendVerificationRequest(
        @NotNull(message = "type 不能为空")
        VerificationType type,

        @NotNull(message = "purpose 不能为空")
        VerificationPurpose purpose,

        @NotBlank(message = "target 不能为空")
        String target
) {
}
