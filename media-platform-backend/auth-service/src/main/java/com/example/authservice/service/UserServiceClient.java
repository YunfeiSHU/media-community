package com.example.authservice.service;

import org.apache.dubbo.config.annotation.DubboReference;
import org.springframework.stereotype.Component;

import com.example.userservice.rpc.CreateUserCommand;
import com.example.userservice.rpc.RpcUser;
import com.example.userservice.rpc.UserDubboService;

/**
 * user-service 的 Dubbo 消费者封装。
 *
 * <p>{@code check = false} 表示启动时不校验提供者是否在线：注册链路允许“用户中心暂时不可用”，
 * 由 {@code AuthService.registerByContact} 把 RPC 异常转成 50000 业务错误。
 * 直连排查时可临时给 {@code @DubboReference} 加 {@code url = "dubbo://127.0.0.1:20881"} 指定端口。
 */
@Component
public class UserServiceClient {
    @DubboReference(
            check = false,
            timeout = 3000
    )
    private UserDubboService userDubboService;

    /**
     * 创建用户档案。
     *
     * <p>注册阶段只确定了登录凭证（手机号 / 邮箱），昵称 / 头像 / 性别 / 简介都还没有，
     * 所以这里传一个空命令，只让 user-service 生成 uid 并落一条 status = NORMAL 的默认资料；
     * 资料由用户随后在个人中心 {@code PUT /api/v1/users/me} 补充（此时 nickname 为空，前端需自行兜底展示）。
     */
    public RpcUser createUser() {
        return userDubboService.createUser(new CreateUserCommand());
    }

    public RpcUser getUser(String uid) {
        return userDubboService.getUser(uid);
    }
}