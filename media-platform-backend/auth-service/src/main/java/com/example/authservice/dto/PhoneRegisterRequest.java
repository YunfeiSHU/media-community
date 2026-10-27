package com.example.authservice.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Pattern;
import jakarta.validation.constraints.Size;

public record PhoneRegisterRequest(
        @NotBlank(message = "phone 不能为空")
        @Pattern(regexp = "^1[3-9]\\d{9}$", message = "phone 格式不正确")
        String phone,

        @NotBlank(message = "code 不能为空")
        @Pattern(regexp = "^\\d{6}$", message = "验证码必须为 6 位数字")
        String code,

        @NotBlank(message = "password 不能为空")
        @Size(min = 8, max = 20, message = "password 长度必须为 8~20 个字符")
        String password
) {
}
