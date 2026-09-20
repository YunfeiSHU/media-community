package com.example.userservice.service;

import jakarta.servlet.http.HttpServletRequest;

public final class CurrentUser {
    private CurrentUser() {
    }

    public static Long id(HttpServletRequest request) {
        String value = request.getHeader("X-User-Id");
        if (value == null || value.isBlank()) {
            throw new IllegalArgumentException("missing X-User-Id header");
        }
        try {
            return Long.valueOf(value);
        } catch (NumberFormatException exception) {
            throw new IllegalArgumentException("invalid X-User-Id header");
        }
    }

    public static void requireAdmin(HttpServletRequest request) {
        if (!"ADMIN".equalsIgnoreCase(request.getHeader("X-User-Role"))) {
            throw new SecurityException("admin role required");
        }
    }
}
