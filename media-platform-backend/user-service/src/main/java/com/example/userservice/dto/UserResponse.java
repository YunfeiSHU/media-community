package com.example.userservice.dto;

import java.time.LocalDateTime;

import com.example.userservice.entity.User;
import com.example.userservice.entity.UserStatus;

/**
 * 用户视图对象。
 *
 * <p>认证信息（手机号 / 邮箱 / 密码）不在本服务存储，也不对外暴露。
 *
 * <p>注意本服务与表里都<b>没有 username 字段</b>：早期实现里名为 {@code username} 的字段
 * 实际返回的是 uid（而真正的 uid 反而没返回），现已更名为 {@link #uid}。
 * {@code userId} 是内部自增主键，仅管理端接口使用。
 */
public record UserResponse(
        Long userId,
        String uid,
        String nickname,
        String avatar,
        String gender,
        String bio,
        UserStatus status,
        LocalDateTime createdAt,
        LocalDateTime updatedAt,
        /** 链路角色名：ADMIN / user。前端用它决定是否显示管理入口；后端仍是唯一鉴权方。 */
        String role
) {
    public static UserResponse from(User user) {
        return new UserResponse(user.getId(), user.getUid(), user.getNickname(), user.getAvatar(),
                user.getGender(), user.getBio(), user.getStatusEnum(), user.getCreatedAt(), user.getUpdatedAt(),
                user.getRoleEnum().roleName());
    }
}
