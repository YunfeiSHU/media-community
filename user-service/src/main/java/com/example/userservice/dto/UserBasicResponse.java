package com.example.userservice.dto;

import com.example.userservice.entity.User;
import com.example.userservice.entity.UserStatus;

public record UserBasicResponse(Long userId, String nickname, String avatar, UserStatus status) {
    public static UserBasicResponse from(User user) {
        return new UserBasicResponse(user.getUserId(), user.getNickname(), user.getAvatar(), user.getStatusEnum());
    }
}
