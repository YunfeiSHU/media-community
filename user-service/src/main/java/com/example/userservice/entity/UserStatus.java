package com.example.userservice.entity;

/**
 * 用户状态枚举（入库值为 TINYINT）。
 *
 * <p>对客契约里这个字段是<b>枚举名</b>（如 {@code "DISABLED"}），
 * 所以 HTTP 接口收到未知状态名时由 Jackson 直接报请求体解析失败（40001），
 * 而 {@link #fromCode(Integer)} 是“读库”方向的兜底：未知数字统一当作 {@link #NORMAL}，不抛异常。
 */
public enum UserStatus {
    NORMAL(1, "正常"),
    DISABLED(2, "禁用"),
    FROZEN(3, "冻结"),
    DELETED(4, "删除");

    private final Integer code;
    private final String name;

    UserStatus(Integer code, String name) {
        this.code = code;
        this.name = name;
    }

    public Integer getCode() {
        return code;
    }

    public String getName() {
        return name;
    }

    public static UserStatus fromCode(Integer code) {
        if (code == null) {
            return NORMAL;
        }
        for (UserStatus status : values()) {
            if (status.code.equals(code)) {
                return status;
            }
        }
        return NORMAL;
    }
}
