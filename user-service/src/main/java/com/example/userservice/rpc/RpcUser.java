package com.example.userservice.rpc;

import java.io.Serializable;

import com.example.userservice.entity.User;

public class RpcUser implements Serializable {
    private String uid;
    private Integer role;

    public static RpcUser from(User user) {
        if (user == null) {
            return null;
        }
        RpcUser result = new RpcUser();
        result.uid = user.getUid();
        result.role = user.getRole();
        return result;
    }

    public String getUid() { return uid; }
    public void setUid(String uid) { this.uid = uid; }
    
    public Integer getRole() { return role; }
    public void setRole(Integer role) { this.role = role; }
}
