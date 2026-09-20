package com.example.userservice.dto;

import com.example.userservice.entity.User;
import com.example.userservice.entity.UserStatus;

import java.time.LocalDateTime;

public record UserResponse(
        Long userId,
        String username,
        String nickname,
        String avatar,
        String gender,
        String bio,
        UserStatus status,
        LocalDateTime createdAt,
        LocalDateTime updatedAt
) {
    public static UserResponse from(User user) {
        return new UserResponse(user.getUserId(), user.getUsername(), user.getNickname(), user.getAvatar(),
                user.getGender(), user.getBio(), user.getStatus(), user.getCreatedAt(), user.getUpdatedAt());
    }
}
