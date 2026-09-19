package com.example.authservice.dto;

public record LoginResponse(
        String uid,
        String accessToken,
        String refreshToken,
        long accessTokenExpiresIn,
        long refreshTokenExpiresIn
) {
}