package com.example.userservice.entity;

import java.time.LocalDateTime;

public class User {
    private Long id;
    private String uid;
    private String nickname;
    private String avatar;
    private String gender;
    private String bio;
    private Integer role;
    private Integer status;
    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;

    public Long getId() { return id; }
    public void setId(Long id) { this.id = id; }
    public String getUid() { return uid; }
    public void setUid(String uid) { this.uid = uid; }
    public Long getUserId() { return id; }
    public void setUserId(Long userId) { this.id = userId; }
    public String getUsername() { return uid; }
    public void setUsername(String username) { this.uid = username; }
    public String getNickname() { return nickname; }
    public void setNickname(String nickname) { this.nickname = nickname; }
    public String getAvatar() { return avatar; }
    public void setAvatar(String avatar) { this.avatar = avatar; }
    public String getGender() { return gender; }
    public void setGender(String gender) { this.gender = gender; }
    public String getBio() { return bio; }
    public void setBio(String bio) { this.bio = bio; }

    public Integer getRole() { return role; }
    public void setRole(Integer role) { this.role = role; }

    public UserRole getRoleEnum() {
        return UserRole.fromCode(role);
    }
    public void setRoleEnum(UserRole roleEnum) {
        this.role = roleEnum != null ? roleEnum.getCode() : UserRole.USER.getCode();
    }

    public Integer getStatus() { return status; }
    public void setStatus(Integer status) { this.status = status; }

    public UserStatus getStatusEnum() {
        return UserStatus.fromCode(status);
    }
    public void setStatusEnum(UserStatus statusEnum) {
        this.status = statusEnum != null ? statusEnum.getCode() : UserStatus.NORMAL.getCode();
    }
    public LocalDateTime getCreatedAt() { return createdAt; }
    public void setCreatedAt(LocalDateTime createdAt) { this.createdAt = createdAt; }
    public LocalDateTime getUpdatedAt() { return updatedAt; }
    public void setUpdatedAt(LocalDateTime updatedAt) { this.updatedAt = updatedAt; }
}
