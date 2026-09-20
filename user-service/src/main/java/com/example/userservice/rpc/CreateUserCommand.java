package com.example.userservice.rpc;

import java.io.Serializable;

public class CreateUserCommand implements Serializable {
    private String username;
    private String nickname;
    private String avatar;
    private String gender;
    private String bio;

    public CreateUserCommand() {
    }

    public CreateUserCommand(String username, String nickname, String avatar, String gender, String bio) {
        this.username = username;
        this.nickname = nickname;
        this.avatar = avatar;
        this.gender = gender;
        this.bio = bio;
    }

    public String getUsername() { return username; }
    public void setUsername(String username) { this.username = username; }
    public String getNickname() { return nickname; }
    public void setNickname(String nickname) { this.nickname = nickname; }
    public String getAvatar() { return avatar; }
    public void setAvatar(String avatar) { this.avatar = avatar; }
    public String getGender() { return gender; }
    public void setGender(String gender) { this.gender = gender; }
    public String getBio() { return bio; }
    public void setBio(String bio) { this.bio = bio; }
}
