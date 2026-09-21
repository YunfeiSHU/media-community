package com.example.userservice.entity;

/**
 * 用户角色枚举
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

    public String getName() {
        return name;
    }

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

    public static String toRoleName(Integer code) {
        return fromCode(code).getName();
    }
}
