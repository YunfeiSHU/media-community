package com.example.authservice.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record RegisterRequest(
        @NotBlank(message = "username 不能为空")
        @Size(max = 32, message = "username 长度不能超过 32 个字符")
        String username,

        @NotBlank(message = "password 不能为空")
        @Size(min = 8, max = 20, message = "password 长度必须为 8~20 个字符")
        String password
) {
}
