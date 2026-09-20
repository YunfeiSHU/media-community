package com.example.authservice.service;

import org.apache.dubbo.config.annotation.DubboReference;
import org.springframework.stereotype.Component;

import com.example.userservice.rpc.CreateUserCommand;
import com.example.userservice.rpc.RpcUser;
import com.example.userservice.rpc.UserDubboService;

@Component
public class UserServiceClient {
    @DubboReference(
            check = false,
            timeout = 3000
    )
    private UserDubboService userDubboService;

    public RpcUser createUser() {
        return userDubboService.createUser(new CreateUserCommand());
    }

    public RpcUser getUser(String uid) {
        return userDubboService.getUser(uid);
    }
}