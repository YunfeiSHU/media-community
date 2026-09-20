package com.example.userservice.dto;

import com.example.userservice.entity.UserStatus;

public record UserStatusResponse(Long userId, UserStatus status) {
}