package com.example.userservice.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record UserCreateRequest(
        @NotBlank @Size(max = 64) String username,
        @NotBlank @Size(max = 64) String nickname,
        @Size(max = 512) String avatar,
        @Size(max = 16) String gender,
        @Size(max = 500) String bio
) {
}
