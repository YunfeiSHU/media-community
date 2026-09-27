package com.example.product.common;

import jakarta.servlet.http.HttpServletRequest;

/**
 * 当前登录用户上下文：只从网关注入的身份头解析，服务本身不解析 JWT。
 *
 * <p>鉴权统一在 gateway-service 完成：JWT 校验通过后，网关把 JWT 标准 claim {@code sub}
 * （即用户 uid）与 {@code role} claim（即角色）分别写入 {@code X-User-Id} / {@code X-User-Role}（白名单路径会先清掉这些头）。
 * 本服务<b>只信任</b>这两个头，<b>不接受</b>请求体 / 查询参数里客户端自报的用户标识与角色，
 * 与 order-service、user-service 的 {@code CurrentUser} 保持同一约定。
 *
 * <p>早期网关曾注入过非标准的 {@code uid} 头，该头<b>已废弃</b>：网关现在只会删除它、不再写入，
 * 因此本类也不再兼容读取——否则任何客户端都能用一个自造的头冒充任意用户。
 *
 * <p>需注意本约定的前提是“请求确实来自网关”：若直连 8003 端口绕过网关，
 * 伪造 {@code X-User-Id} / {@code X-User-Role} 依旧可以冒充身份，生产环境必须在网络层禁止直连。
 */
public final class CurrentUser {

    /** 网关注入的用户标识头（值 = JWT 的 sub） */
    private static final String USER_ID_HEADER = "X-User-Id";

    /** 网关注入的角色头（值 = JWT 的 role claim） */
    private static final String USER_ROLE_HEADER = "X-User-Role";

    /** 管理员角色名，与 auth-service 签发 token 时写入的 role claim 取值一致 */
    private static final String ADMIN_ROLE = "ADMIN";

    /** 与 product.uid 字段长度（VARCHAR(32)）保持一致 */
    private static final int MAX_UID_LENGTH = 32;

    private CurrentUser() {
    }

    /**
     * 获取当前用户 uid。
     *
     * @throws BizException 缺少身份头（40101）或 uid 超长（40001）
     */
    public static String uid(HttpServletRequest request) {
        return requireUid(request.getHeader(USER_ID_HEADER));
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
     * <p>当前商品写接口只要求登录（未按角色鉴权）；如后续需要限制为管理员，
     * 在控制器里调用本方法即可，不要另开角色解析路径。
     *
     * @throws BizException 缺少身份头（40101）或角色不是 ADMIN（40301）
     */
    public static void requireAdmin(HttpServletRequest request) {
        requireRole(request, ADMIN_ROLE);
    }

    /**
     * 校验并规整 uid（也供需要显式传入 uid 的调用方复用）。
     *
     * @throws BizException 为空（40101）或超长（40001）
     */
    public static String requireUid(String uid) {
        if (uid == null || uid.isBlank()) {
            throw new BizException(ErrorCode.UNAUTHORIZED, "缺少用户身份信息，请通过网关登录后访问");
        }
        String trimmed = uid.trim();
        if (trimmed.length() > MAX_UID_LENGTH) {
            throw new BizException(ErrorCode.INVALID_PARAM, "用户标识长度不能超过" + MAX_UID_LENGTH);
        }
        return trimmed;
    }
}
