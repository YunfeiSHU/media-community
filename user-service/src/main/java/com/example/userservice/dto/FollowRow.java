package com.example.userservice.dto;

import java.time.LocalDateTime;

/**
 * 关注列表里的一行：用户公开资料 + 关注时间。
 *
 * <p>用可变 POJO 而非 record：MyBatis 注解 SQL 的结果映射走 setter，
 * 列名（snake_case）由 {@code map-underscore-to-camel-case} 转成属性名。
 */
public class FollowRow {

    private String uid;

    private String nickname;

    private String avatar;

    private String bio;

    private String gender;

    private LocalDateTime createdAt;

    public String getUid() { return uid; }
    public void setUid(String uid) { this.uid = uid; }

    public String getNickname() { return nickname; }
    public void setNickname(String nickname) { this.nickname = nickname; }

    public String getAvatar() { return avatar; }
    public void setAvatar(String avatar) { this.avatar = avatar; }

    public String getBio() { return bio; }
    public void setBio(String bio) { this.bio = bio; }

    public String getGender() { return gender; }
    public void setGender(String gender) { this.gender = gender; }

    public LocalDateTime getCreatedAt() { return createdAt; }
    public void setCreatedAt(LocalDateTime createdAt) { this.createdAt = createdAt; }
}
