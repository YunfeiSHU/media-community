package com.example.authservice.controller;

import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

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
        loginRateLimitService.check(httpRequest.getRemoteAddr());
        return ApiResponse.success(authService.login(request));
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

    @PostMapping("/force-logout")
    public ApiResponse<Void> forceLogout(@RequestParam("uid") String uid,
                                        @RequestHeader("Authorization") String authorization) {
        authService.forceLogout(uid, authorization);
        return ApiResponse.success();
    }

    @PostMapping("/verification/send")
    public ApiResponse<VerificationInfo> sendVerification(@Valid @RequestBody SendVerificationRequest request) {
        String code = verificationService.send(request.purpose(), request.type(), request.target());
        // For email, code is null (sent via email for security). For phone, code is returned.
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

    public record ApiResponse<T>(int code, String message, T data) {
        private static <T> ApiResponse<T> success() {
            return new ApiResponse<>(0, "success", null);
        }

        private static <T> ApiResponse<T> success(T data) {
            return new ApiResponse<>(0, "success", data);
        }
    }

    public record VerificationInfo(String purpose, String type, String target, String code) {
    }
}
