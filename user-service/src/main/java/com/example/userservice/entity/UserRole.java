package com.example.userservice.entity;

/**
 * 用户角色枚举（值对应占 {f 1} 字节的数字码）。
 *
 * <p>注意区分两套“名字”：
 * <ul>
 *     <li>{@code code}：入库值（0 = 管理员，1 = 普通用户），与 {@code user} 表的 role 列一致；</li>
 *     <li>{@code name}：面向人的中文描述，只用于展示。</li>
 * </ul>
 * 真正的角色名（{@code "ADMIN"} / {@code "user"}）由 auth-service 签发 JWT 时按 code 映射得到
 * （见 `JwtTokenService#generateAccessToken`），网关再原样写入 {@code X-User-Role}。
 * 因此本类曾经提供的 {@code toRoleName()}（返回“管理员”/“用户”）与链路里实际用的角色名不一致，已删掉，
 * 以免有人把它当角色名去拼 {@code X-User-Role}。
 */
public enum UserRole {
    ADMIN(0, "管理员"),
    USER(1, "用户");

    private final Integer code;
    private final String name;

    UserRole(Integer code, String name) {
        this.code = code;
        this.name = name;
    }

    public Integer getCode() {
        return code;
    }

    /** 中文展示名（<b>不是</b> X-User-Role 里的角色名） */
    public String getName() {
        return name;
    }

    /** 按入库值反查；未知值回落到 {@link #USER}，不抛异常 */
    public static UserRole fromCode(Integer code) {
        if (code == null) {
            return USER;
        }
        for (UserRole role : values()) {
            if (role.code.equals(code)) {
                return role;
            }
        }
        return USER;
    }

    /**
     * 链路里的角色名：{@code ADMIN} / {@code user}。
     *
     * <p>必须与 auth-service 签发 JWT 时写入 {@code role} claim 的映射一致
     * （见 {@code JwtTokenService#generateAccessToken}：code 0 → ADMIN，其余 → user），
     * 也与网关写入的 {@code X-User-Role} 一致。前端用它做 UI 权限判断。
     */
    public String roleName() {
        return this == ADMIN ? "ADMIN" : "user";
    }
}
