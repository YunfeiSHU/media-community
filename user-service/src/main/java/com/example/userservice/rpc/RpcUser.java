package com.example.userservice.rpc;

import java.io.Serializable;
import java.time.LocalDateTime;

import com.example.userservice.entity.User;
import com.example.userservice.entity.UserStatus;

public class RpcUser implements Serializable {
    private String uid;
    private String nickname;
    private String avatar;
    private String gender;
    private String bio;
    private UserStatus status;
    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;

    public static RpcUser from(User user) {
        RpcUser result = new RpcUser();
        result.uid = user.getUid();
        result.nickname = user.getNickname();
        result.avatar = user.getAvatar();
        result.gender = user.getGender();
        result.bio = user.getBio();
        result.status = user.getStatus();
        result.createdAt = user.getCreatedAt();
        result.updatedAt = user.getUpdatedAt();
        return result;
    }

    public String getUid() { return uid; }
    public String getNickname() { return nickname; }
    public String getAvatar() { return avatar; }
    public String getGender() { return gender; }
    public String getBio() { return bio; }
    public UserStatus getStatus() { return status; }
    public LocalDateTime getCreatedAt() { return createdAt; }
    public LocalDateTime getUpdatedAt() { return updatedAt; }
}
