package com.example.userservice.entity;

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
