package com.example.order.common;

import jakarta.servlet.http.HttpServletRequest;

/**
 * 当前登录用户：从网关注入的身份头中解析 uid。
 *
 * <p>鉴权在网关完成（JWT 校验通过后注入 {@code uid} / {@code X-User-Id} / {@code X-User-Role}，
 * 白名单路径会清掉这些头），服务本身不做 JWT 校验。与 user-service 的 {@code CurrentUser} 保持同一约定。
 */
public final class CurrentUser {

    /** 网关注入的用户标识头（与 X-User-Id 同值） */
    private static final String UID_HEADER = "uid";

    private static final String USER_ID_HEADER = "X-User-Id";

    private CurrentUser() {
    }

    /**
     * 获取当前用户 UID。
     *
     * @throws BizException 缺少身份头（未登录 / 未经网关访问）时抛出 40101
     */
    public static String uid(HttpServletRequest request) {
        String value = request.getHeader(UID_HEADER);
        if (value == null || value.isBlank()) {
            value = request.getHeader(USER_ID_HEADER);
        }
        if (value == null || value.isBlank()) {
            throw new BizException(ErrorCode.UNAUTHORIZED, "缺少用户身份信息，请通过网关登录后访问");
        }
        return value.trim();
    }
}
