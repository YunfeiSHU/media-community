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

    /**
     * 当前登录用户的资料。
     *
     * <p>身份取自网关注入的 {@code X-User-Id}（= uid），客户端无法指定查询对象。
     */
    @GetMapping("/users/me")
    public ApiResponse<UserResponse> me(HttpServletRequest request) {
        return ApiResponse.success(UserResponse.from(userService.getRequiredByUid(CurrentUser.uid(request))));
    }

    /** 更新当前登录用户的资料（字段全部可选，null 表示不修改）。 */
    @PutMapping("/users/me")
    public ApiResponse<UserResponse> updateMe(@Valid @RequestBody UserUpdateRequest updateRequest,
                                             HttpServletRequest request) {
        return ApiResponse.success(
                UserResponse.from(userService.updateProfileByUid(CurrentUser.uid(request), updateRequest)));
    }

    /**
     * 注销当前账号（逻辑删除，把状态置为 DELETED）。
     *
     * <p>只改本服务的资料状态：auth 侧的账号与 Refresh Token 不在本事务内，需要上层补偿。
     */
    @DeleteMapping("/users/me")
    public ApiResponse<Void> deleteMe(HttpServletRequest request) {
        userService.updateStatusByUid(CurrentUser.uid(request), UserStatus.DELETED);
        return ApiResponse.success();
    }

    /**
     * 管理端按内部主键查询用户。
     *
     * <p>注意路径里的 {@code userId} 是 {@code user}.id（自增主键），不是对外的 uid；
     * 对客身份一律用 uid，避免把内部主键泄露到业务链路里。
     */
    @GetMapping("/admin/users/{userId}")
    public ApiResponse<UserResponse> getById(@PathVariable Long userId, HttpServletRequest request) {
        CurrentUser.requireAdmin(request);
        return ApiResponse.success(UserResponse.from(userService.getRequired(userId)));
    }

    /** 管理端修改用户状态。同理，{@code userId} 是内部主键。 */
    @PatchMapping("/admin/users/{userId}/status")
    public ApiResponse<UserResponse> updateStatus(@PathVariable Long userId,
                                                  @Valid @RequestBody UserStatusRequest statusRequest,
                                                  HttpServletRequest request) {
        CurrentUser.requireAdmin(request);
        return ApiResponse.success(UserResponse.from(userService.updateStatus(userId, statusRequest.status())));
    }
}
