package com.example.userservice.controller;

import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PatchMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import com.example.userservice.common.ApiResponse;
import com.example.userservice.common.CurrentUser;
import com.example.userservice.dto.UserResponse;
import com.example.userservice.dto.UserStatusRequest;
import com.example.userservice.dto.UserUpdateRequest;
import com.example.userservice.entity.UserStatus;
import com.example.userservice.service.UserService;

import jakarta.servlet.http.HttpServletRequest;
import jakarta.validation.Valid;

/**
 * 用户 HTTP 接口。
 *
 * <p>调用者身份只来自网关注入的 {@code X-User-Id}（JWT 的 {@code sub}），
 * 角色只来自 {@code X-User-Role}（JWT 的 {@code role} claim）；
 * 响应统一为 {@link ApiResponse} 结构（与 auth / product / order 服务一致）。
 */
@RestController
@RequestMapping("/api/v1")
public class UserController {
    private final UserService userService;

    public UserController(UserService userService) {
        this.userService = userService;
    }

    @GetMapping("/users/me")
    public ApiResponse<UserResponse> me(HttpServletRequest request) {
        return ApiResponse.success(UserResponse.from(userService.getRequiredByUid(CurrentUser.uid(request))));
    }

    @PutMapping("/users/me")
    public ApiResponse<UserResponse> updateMe(@Valid @RequestBody UserUpdateRequest updateRequest,
                                             HttpServletRequest request) {
        return ApiResponse.success(
                UserResponse.from(userService.updateProfileByUid(CurrentUser.uid(request), updateRequest)));
    }

    @DeleteMapping("/users/me")
    public ApiResponse<Void> deleteMe(HttpServletRequest request) {
        userService.updateStatusByUid(CurrentUser.uid(request), UserStatus.DELETED);
        return ApiResponse.success();
    }

    @GetMapping("/admin/users/{userId}")
    public ApiResponse<UserResponse> getById(@PathVariable Long userId, HttpServletRequest request) {
        CurrentUser.requireAdmin(request);
        return ApiResponse.success(UserResponse.from(userService.getRequired(userId)));
    }

    @PatchMapping("/admin/users/{userId}/status")
    public ApiResponse<UserResponse> updateStatus(@PathVariable Long userId,
                                                  @Valid @RequestBody UserStatusRequest statusRequest,
                                                  HttpServletRequest request) {
        CurrentUser.requireAdmin(request);
        return ApiResponse.success(UserResponse.from(userService.updateStatus(userId, statusRequest.status())));
    }
}
