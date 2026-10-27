package com.example.userservice.common;

import jakarta.servlet.http.HttpServletRequest;

/**
 * 当前登录用户上下文：只从网关注入的身份头解析，服务本身不解析 JWT。
 *
 * <p>鉴权统一在 gateway-service 完成：JWT 校验通过后，网关把 JWT 标准 claim {@code sub}
 * （即用户 uid）与 {@code role} claim（即角色）分别写入 {@code X-User-Id} / {@code X-User-Role}；白名单路径会先清掉这些头。
 * 因此本服务<b>只信任</b>这两个头，绝不接受请求体 / 查询参数里客户端自报的用户标识或角色。
 *
 * <p>早期网关曾注入过非标准的 {@code uid} 头，该头<b>已废弃</b>：网关现在只会删除它、不再写入，
 * 因此本类也不再兼容读取——否则任何客户端都能用一个自造的头冒充任意用户。
 *
 * <p>需注意本约定的前提是“请求确实来自网关”：若直连 8001 端口绕过网关，
 * 伪造 {@code X-User-Id} / {@code X-User-Role} 依旧可以冒充身份，生产环境必须在网络层禁止直连。
 */
public final class CurrentUser {

    /** 网关注入的用户标识头（值 = JWT 的 sub） */
    private static final String USER_ID_HEADER = "X-User-Id";

    /** 网关注入的角色头（值 = JWT 的 role claim） */
    private static final String USER_ROLE_HEADER = "X-User-Role";

    /** 管理员角色名，与 auth-service 签发 token 时写入的 role claim 取值一致 */
    private static final String ADMIN_ROLE = "ADMIN";

    private CurrentUser() {
    }

    /**
     * 获取当前用户 uid。
     *
     * @throws BizException 缺少身份头（未登录 / 未经网关访问），错误码 40101
     */
    public static String uid(HttpServletRequest request) {
        String value = request.getHeader(USER_ID_HEADER);
        if (value == null || value.isBlank()) {
            throw new BizException(ErrorCode.UNAUTHORIZED, "缺少用户身份信息，请通过网关登录后访问");
        }
        return value.trim();
    }

    /**
     * 获取当前用户角色。
     *
     * <p>唯一来源是网关注入的 {@code X-User-Role}（值 = JWT 的 {@code role} claim）；
     * 服务不解析 JWT，也不接受请求体 / 查询参数 / 其它头里自报的角色。
     *
     * @return 角色名；未注入时返回 {@code null}
     */
    public static String role(HttpServletRequest request) {
        String value = request.getHeader(USER_ROLE_HEADER);
        return value == null || value.isBlank() ? null : value.trim();
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
