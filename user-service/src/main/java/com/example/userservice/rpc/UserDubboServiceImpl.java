package com.example.userservice.rpc;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.stream.Collectors;

import org.apache.dubbo.config.annotation.DubboService;

import com.example.userservice.entity.User;
import com.example.userservice.service.UserService;

@DubboService
public class UserDubboServiceImpl implements UserDubboService {
    private final UserService userService;

    public UserDubboServiceImpl(UserService userService) {
        this.userService = userService;
    }

    /**
     * 创建用户档案。
     *
     * <p>赋值逻辑与 HTTP 入口统一放在 {@link UserService#createUser}，本方法只做参数校验与协议转换，
     * 避免两份代码在 role/status 上再次走偏。
     */
    @Override
    public RpcUser createUser(CreateUserCommand command) {
        if (command == null) {
            throw new IllegalArgumentException("create command is required");
        }
        User user = userService.createUser(command.getNickname(), command.getAvatar(),
                command.getGender(), command.getBio());
        return RpcUser.from(user);
    }

    @Override
    public RpcUser getUser(String uid) {
        return RpcUser.from(userService.getRequiredByUid(uid));
    }

    @Override
    public List<RpcUser> batchGetUsers(List<String> uids) {
        return userService.getByUids(uids == null ? List.of() : uids).stream()
                .map(RpcUser::from)
                .toList();
    }

    @Override
    public Map<String, Boolean> checkUserExists(List<String> uids) {
        List<String> ids = uids == null ? List.of() : uids;
        Set<String> existingIds = userService.getByUids(ids).stream()
                .map(User::getUid)
                .collect(Collectors.toSet());
        Map<String, Boolean> result = new LinkedHashMap<>();
        ids.forEach(id -> result.put(id, existingIds.contains(id)));
        return result;
    }

}
