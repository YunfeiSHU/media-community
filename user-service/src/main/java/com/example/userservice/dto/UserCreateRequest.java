package com.example.userservice.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

/**
 * 创建用户档案的请求体。
 *
 * <p>每个约束都带中文 message：{@code GlobalExceptionHandler} 取的是 {@code defaultMessage}，
 * 不写 message 就会把 {@code must not be blank} 这类英文直接颖给前端（与 auth-service 的 DTO 保持一致）。
 *
 * <p>{@code bio} 上限与表结构 `user`.bio（VARCHAR(500)）对齐；
 * 早期写 500 而建表是 255，会让“前端校验通过、入库报 Data too long”变成 500。
 */
public record UserCreateRequest(
        @NotBlank(message = "nickname 不能为空")
        @Size(max = 64, message = "nickname 长度不能超过 64") String nickname,

        @Size(max = 512, message = "avatar 长度不能超过 512") String avatar,

        @Size(max = 16, message = "gender 长度不能超过 16") String gender,

        @Size(max = 500, message = "bio 长度不能超过 500") String bio
) {
}
