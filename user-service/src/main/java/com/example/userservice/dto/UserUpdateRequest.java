package com.example.userservice.dto;

import jakarta.validation.constraints.Size;

/**
 * 更新个人资料的请求体（全部可选）。
 *
 * <p>各字段上限与表结构对齐（`user`.nickname 64 / avatar 512 / bio 500）。
 * {@code bio} 与 {@code gender} 曾经分别是 500/16 而建表是 255 与 TINYINT，
 * 导致“校验通过但入库报错”——现已把类型与长度两边对齐。
 */
public record UserUpdateRequest(
        @Size(max = 64, message = "nickname 长度不能超过 64") String nickname,

        @Size(max = 512, message = "avatar 长度不能超过 512") String avatar,

        @Size(max = 16, message = "gender 长度不能超过 16") String gender,

        @Size(max = 500, message = "bio 长度不能超过 500") String bio
) {
}
