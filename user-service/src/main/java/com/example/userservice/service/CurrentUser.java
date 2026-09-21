package com.example.userservice.service;

import jakarta.servlet.http.HttpServletRequest;

public final class CurrentUser {
    private CurrentUser() {
    }

    public static String uid(HttpServletRequest request) {
        String value = request.getHeader("uid");
        if (value == null || value.isBlank()) {
            value = request.getHeader("X-User-Id");
        }
        if (value == null || value.isBlank()) {
            throw new IllegalArgumentException("missing uid header");
        }
        return value;
    }

    public static void requireAdmin(HttpServletRequest request) {
        String role = request.getHeader("X-User-Role");
        if (role == null || role.isBlank()) {
            throw new SecurityException("admin role required");
        }
        if (!"ADMIN".equalsIgnoreCase(role)) {
            throw new SecurityException("admin role required");
        }
    }
}
