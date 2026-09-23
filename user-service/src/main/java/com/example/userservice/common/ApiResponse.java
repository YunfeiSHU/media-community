package com.example.userservice.common;

/**
 * 统一响应结构（与 auth / product / order 服务保持一致）。
 *
 * @param code    0 表示成功，其它为 {@link ErrorCode}
 * @param message 描述信息
 * @param data    业务数据，失败时为 null
 */
public record ApiResponse<T>(int code, String message, T data) {

    public static <T> ApiResponse<T> success(T data) {
        return new ApiResponse<>(0, "success", data);
    }

    public static ApiResponse<Void> success() {
        return new ApiResponse<>(0, "success", null);
    }

    public static <T> ApiResponse<T> failure(int code, String message) {
        return new ApiResponse<>(code, message, null);
    }
}
