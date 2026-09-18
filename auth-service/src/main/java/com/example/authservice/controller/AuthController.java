package com.example.authservice.controller;

import com.example.authservice.dto.LoginRequest;
import com.example.authservice.dto.RegisterRequest;
import com.example.authservice.service.AuthService;
import jakarta.validation.Valid;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.Map;

@RestController
@RequestMapping("/api/v1/auth")
public class AuthController {

    private final AuthService authService;

    public AuthController(AuthService authService) {
        this.authService = authService;
    }

    @PostMapping("/register")
    public ApiResponse<Void> register(@Valid @RequestBody RegisterRequest request) {
        authService.register(request);
        return ApiResponse.success();
    }

    @PostMapping("/login")
    public ApiResponse<Void> login(@Valid @RequestBody LoginRequest request) {
        authService.login(request);
        return ApiResponse.success();
    }

    public record ApiResponse<T>(int code, String message, T data) {
        private static <T> ApiResponse<T> success() {
            return new ApiResponse<>(0, "success", null);
        }
    }
}
