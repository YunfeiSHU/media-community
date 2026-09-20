package com.example.userservice.dto;

import jakarta.validation.constraints.Size;

public record UserUpdateRequest(
        @Size(max = 64) String nickname,
        @Size(max = 512) String avatar,
        @Size(max = 16) String gender,
        @Size(max = 500) String bio
) {
}
