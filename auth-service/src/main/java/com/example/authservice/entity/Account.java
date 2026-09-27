package com.example.authservice.entity;

import java.time.LocalDateTime;

/**
 * 账号实体，对应表 {@code account}。
 *
 * <p>{@link #username} 在当前链路里始终为 NULL：注册只支持手机号 / 邮箱，没有“账号名”。
 * 保留字段与 {@code uk_username} 唯一索引，是为了将来支持用户名登录；唯一索引允许多个 NULL，不会冲突。
 * 对外的用户唯一标识是 {@link #uid}（由 user-service 生成，登录时写入 JWT 的 {@code sub}）。
 */
public class Account {

    private Long id;
    private String uid;
    private String username;
    private String phone;
    private String email;
    private String passwordHash;
    private Integer status;
    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;

    public Long getId() {
        return id;
    }

    public String getUid() {
        return uid;
    }

    public void setUid(String uid) {
        this.uid = uid;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public String getUsername() {
        return username;
    }

    public void setUsername(String username) {
        this.username = username;
    }

    public String getPhone() {
        return phone;
    }

    public void setPhone(String phone) {
        this.phone = phone;
    }

    public String getEmail() {
        return email;
    }

    public void setEmail(String email) {
        this.email = email;
    }

    public String getPasswordHash() {
        return passwordHash;
    }

    public void setPasswordHash(String passwordHash) {
        this.passwordHash = passwordHash;
    }

    public Integer getStatus() {
        return status;
    }

    public void setStatus(Integer status) {
        this.status = status;
    }

    public LocalDateTime getCreatedAt() {
        return createdAt;
    }

    public void setCreatedAt(LocalDateTime createdAt) {
        this.createdAt = createdAt;
    }

    public LocalDateTime getUpdatedAt() {
        return updatedAt;
    }

    public void setUpdatedAt(LocalDateTime updatedAt) {
        this.updatedAt = updatedAt;
    }
}
