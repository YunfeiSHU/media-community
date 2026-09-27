package com.example.authservice.controller;

import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import com.example.authservice.common.ApiResponse;
import com.example.authservice.common.CurrentUser;
import com.example.authservice.dto.EmailRegisterRequest;
import com.example.authservice.dto.LoginRequest;
import com.example.authservice.dto.LoginResponse;
import com.example.authservice.dto.PhoneRegisterRequest;
import com.example.authservice.dto.RefreshTokenRequest;
import com.example.authservice.dto.SendVerificationRequest;
import com.example.authservice.service.AuthService;
import com.example.authservice.service.LoginRateLimitService;
import com.example.authservice.service.VerificationService;
import com.example.authservice.verification.enums.VerificationType;

import jakarta.servlet.http.HttpServletRequest;
import jakarta.validation.Valid;

@RestController
@RequestMapping("/api/v1/auth")
public class AuthController {

    private final AuthService authService;
    private final LoginRateLimitService loginRateLimitService;
    private final VerificationService verificationService;

    public AuthController(AuthService authService, LoginRateLimitService loginRateLimitService,
                          VerificationService verificationService) {
        this.authService = authService;
        this.loginRateLimitService = loginRateLimitService;
        this.verificationService = verificationService;
    }

    @PostMapping("/login")
    public ApiResponse<LoginResponse> login(@Valid @RequestBody LoginRequest request,
                                            HttpServletRequest httpRequest) {
        // RemoteAddr 在经网关访问时是网关地址，需配合 server.forward-headers-strategy=framework
        // 解析 X-Forwarded-For（见 application.yml），否则所有客户端会共用一个限流桶
        String clientIp = httpRequest.getRemoteAddr();
        loginRateLimitService.checkBeforeLogin(clientIp);
        LoginResponse response = authService.login(request);
        // 登录成功后清零计数：限流只统计「失败尝试」，否则正常用户连登几次就会被误限
        loginRateLimitService.onLoginSuccess(clientIp);
        return ApiResponse.success(response);
    }

    @PostMapping("/token/refresh")
    public ApiResponse<LoginResponse> refresh(@Valid @RequestBody RefreshTokenRequest request) {
        return ApiResponse.success(authService.refresh(request));
    }

    @PostMapping("/logout")
    public ApiResponse<Void> logout(@RequestHeader("Authorization") String authorization) {
        authService.logout(authorization);
        return ApiResponse.success();
    }

    /**
     * 管理员强制下线指定用户。
     *
     * <p>{@code uid} 是<b>操作目标</b>（即被下线的用户）；调用者身份与角色只能来自
     * 网关校验 JWT 后注入的 {@code X-User-Id} / {@code X-User-Role}，非管理员返回 403。
     */
    @PostMapping("/force-logout")
    public ApiResponse<Void> forceLogout(@RequestParam("uid") String targetUid,
                                        @RequestHeader("Authorization") String authorization,
                                        HttpServletRequest httpRequest) {
        CurrentUser.requireAdmin(httpRequest);
        authService.forceLogout(targetUid, authorization);
        return ApiResponse.success();
    }

    @PostMapping("/verification/send")
    public ApiResponse<VerificationInfo> sendVerification(@Valid @RequestBody SendVerificationRequest request) {
        String code = verificationService.send(request.purpose(), request.type(), request.target());
        // 学习模式（auth.verification.expose-code，默认 true）：验证码会随响应体返回，
        // 因为本项目的 Email/SmsNotificationProvider 只是打日志的桩实现，没有真实邮件/短信通道。
        // 生产环境必须置为 false：send() 会返回 null，验证码只留在 Redis 与通知通道里。
        VerificationInfo info = new VerificationInfo(request.purpose().name().toLowerCase(),
                request.type().name().toLowerCase(), request.target(), code);
        return ApiResponse.success(info);
    }

    @PostMapping("/register/phone")
    public ApiResponse<Void> registerPhone(@Valid @RequestBody PhoneRegisterRequest request) {
        authService.registerByContact(VerificationType.PHONE, request.phone(), request.code(), request.password());
        return ApiResponse.success();
    }

    @PostMapping("/register/email")
    public ApiResponse<Void> registerEmail(@Valid @RequestBody EmailRegisterRequest request) {
        authService.registerByContact(VerificationType.EMAIL, request.email(), request.code(), request.password());
        return ApiResponse.success();
    }

    public record VerificationInfo(String purpose, String type, String target, String code) {
    }
}
