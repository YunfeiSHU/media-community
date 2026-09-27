package com.example.userservice.rpc;

import java.io.Serializable;

/**
 * 创建用户档案的 Dubbo 入参（user-service 是契约所有者，auth-service 持有同名副本）。
 *
 * <p><b>没有 username 字段</b>：本系统的登录凭证只有手机号 / 邮箱（见 auth-service 的注册接口），
 * 没有“账号名”这个概念，因此早期文档里“username 由认证侧传入”的说法与实际链路不符。
 * 用户唯一标识是服务端生成的 {@code uid}（auth 侧拿到回值后写进 account.uid）。
 */
public class CreateUserCommand implements Serializable {

    private static final long serialVersionUID = 1L;

    private String nickname;
    private String avatar;
    private String gender;
    private String bio;

    public CreateUserCommand() {
    }

    public CreateUserCommand(String nickname, String avatar, String gender, String bio) {
        this.nickname = nickname;
        this.avatar = avatar;
        this.gender = gender;
        this.bio = bio;
    }

    public String getNickname() { return nickname; }
    public void setNickname(String nickname) { this.nickname = nickname; }
    public String getAvatar() { return avatar; }
    public void setAvatar(String avatar) { this.avatar = avatar; }
    public String getGender() { return gender; }
    public void setGender(String gender) { this.gender = gender; }
    public String getBio() { return bio; }
    public void setBio(String bio) { this.bio = bio; }
}
