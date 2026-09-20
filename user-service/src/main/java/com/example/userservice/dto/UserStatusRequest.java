package com.example.userservice.dto;

import com.example.userservice.entity.UserStatus;
import jakarta.validation.constraints.NotNull;

public record UserStatusRequest(@NotNull UserStatus status) {
}
