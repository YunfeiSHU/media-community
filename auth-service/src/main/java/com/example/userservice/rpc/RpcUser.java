package com.example.userservice.rpc;

import java.io.Serializable;

public class RpcUser implements Serializable {
    private String uid;

    public String getUid() { return uid; }
}