package com.example.userservice.controller;

import com.example.userservice.dto.UserResponse;
import com.example.userservice.dto.UserStatusRequest;
import com.example.userservice.dto.UserUpdateRequest;
import com.example.userservice.service.CurrentUser;
import com.example.userservice.service.UserService;
import jakarta.servlet.http.HttpServletRequest;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PatchMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1")
public class UserController {
    private final UserService userService;

    public UserController(UserService userService) {
        this.userService = userService;
    }

    @GetMapping("/users/me")
    public UserResponse me(HttpServletRequest request) {
        return UserResponse.from(userService.getRequired(CurrentUser.id(request)));
    }

    @PutMapping("/users/me")
    public UserResponse updateMe(@jakarta.validation.Valid @RequestBody UserUpdateRequest updateRequest,
                                 HttpServletRequest request) {
        return UserResponse.from(userService.updateProfile(CurrentUser.id(request), updateRequest));
    }

    @DeleteMapping("/users/me")
    public org.springframework.http.ResponseEntity<Void> deleteMe(HttpServletRequest request) {
        userService.updateStatus(CurrentUser.id(request), com.example.userservice.entity.UserStatus.DELETED);
        return org.springframework.http.ResponseEntity.noContent().build();
    }

    @GetMapping("/admin/users/{userId}")
    public UserResponse getById(@PathVariable Long userId, HttpServletRequest request) {
        CurrentUser.requireAdmin(request);
        return UserResponse.from(userService.getRequired(userId));
    }

    @PatchMapping("/admin/users/{userId}/status")
    public UserResponse updateStatus(@PathVariable Long userId,
                                     @jakarta.validation.Valid @RequestBody UserStatusRequest statusRequest,
                                     HttpServletRequest request) {
        CurrentUser.requireAdmin(request);
        return UserResponse.from(userService.updateStatus(userId, statusRequest.status()));
    }
}
