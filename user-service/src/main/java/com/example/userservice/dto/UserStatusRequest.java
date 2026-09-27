package com.example.userservice.dto;

import com.example.userservice.entity.UserStatus;
import jakarta.validation.constraints.NotNull;

/** 修改用户状态的请求体（管理端接口）。 */
public record UserStatusRequest(@NotNull(message = "status 不能为空") UserStatus status) {
}
