package com.example.userservice.rpc;

import java.io.Serializable;

public class RpcUser implements Serializable {
    private String uid;
    private Integer role;

    public String getUid() { return uid; }
    public void setUid(String uid) { this.uid = uid; }
    
    public Integer getRole() { return role; }
    public void setRole(Integer role) { this.role = role; }
}