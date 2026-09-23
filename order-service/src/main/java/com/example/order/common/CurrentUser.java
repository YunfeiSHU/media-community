package com.example.order.common;

import jakarta.servlet.http.HttpServletRequest;

/**
 * 当前登录用户：从网关注入的身份头中解析 uid。
 *
 * <p>鉴权在网关完成：JWT 校验通过后，网关把 JWT 标准 claim {@code sub}（用户 uid）与
 * {@code role} claim（角色）分别写入 {@code X-User-Id} / {@code X-User-Role}（白名单路径会先清掉这些头），
 * 服务本身不做 JWT 校验。本服务只信任这两个头，不接受请求体 / 查询参数里的用户标识与角色。
 * 与 user-service、product-service 的 {@code CurrentUser} 保持同一约定。
 *
 * <p>兼容历史非标准头 {@code uid}（早期网关注入），仅作兜底读取。
 */
public final class CurrentUser {

    /** 网关注入的用户标识头（值 = JWT 的 sub） */
    private static final String USER_ID_HEADER = "X-User-Id";

    /** 历史非标准头，兜底兼容 */
    private static final String LEGACY_UID_HEADER = "uid";

    /** 网关注入的角色头（值 = JWT 的 role claim） */
    private static final String USER_ROLE_HEADER = "X-User-Role";

    /** 管理员角色名，与 auth-service 签发 token 时写入的 role claim 取值一致 */
    private static final String ADMIN_ROLE = "ADMIN";

    private CurrentUser() {
    }

    /**
     * 获取当前用户角色。
     *
     * <p>唯一来源是网关注入的 {@code X-User-Role}（值 = JWT 的 {@code role} claim）；
     * 服务不解析 JWT，也不接受请求体 / 查询参数 / 其它头里自报的角色。
     * 当前订单接口尚未按角色鉴权，需要在接口上做角色判断时统一从这里取。
     *
     * @return 角色名；未注入时返回 {@code null}
     */
    public static String role(HttpServletRequest request) {
        String value = request.getHeader(USER_ROLE_HEADER);
        return value == null || value.isBlank() ? null : value.trim();
    }

    /**
     * 获取当前用户 UID。
     *
     * @throws BizException 缺少身份头（未登录 / 未经网关访问）时抛出 40101
     */
    public static String uid(HttpServletRequest request) {
        String value = request.getHeader(USER_ID_HEADER);
        if (value == null || value.isBlank()) {
            value = request.getHeader(LEGACY_UID_HEADER);
        }
        if (value == null || value.isBlank()) {
            throw new BizException(ErrorCode.UNAUTHORIZED, "缺少用户身份信息，请通过网关登录后访问");
        }
        return value.trim();
    }

    /**
     * 断言当前用户具备指定角色（忽略大小写）。
     *
     * @throws BizException 缺少身份头（40101）或角色不匹配（40301）
     */
    public static void requireRole(HttpServletRequest request, String expectedRole) {
        uid(request);
        String role = role(request);
        if (role == null || !role.equalsIgnoreCase(expectedRole)) {
            throw new BizException(ErrorCode.FORBIDDEN, "需要" + expectedRole + "权限");
        }
    }

    /**
     * 断言当前用户具备管理员角色。
     *
     * @throws BizException 缺少身份头（40101）或缺少角色头、角色不是 ADMIN（40301）
     */
    public static void requireAdmin(HttpServletRequest request) {
        uid(request);
        String role = role(request);
        if (role == null || !ADMIN_ROLE.equalsIgnoreCase(role)) {
            throw new BizException(ErrorCode.FORBIDDEN, "需要管理员权限");
        }
    }
}
