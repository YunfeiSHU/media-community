package com.example.userservice.rpc;

import java.time.LocalDateTime;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
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

    @Override
    public RpcUser createUser(CreateUserCommand command) {
        if (command == null) {
            throw new IllegalArgumentException("create command is required");
        }
        User user = new User();
        user.setUid(UUID.randomUUID().toString().replace("-", ""));
        user.setNickname(command.getNickname());
        user.setAvatar(command.getAvatar());
        user.setGender(command.getGender());
        user.setBio(command.getBio());
        user.setStatus(com.example.userservice.entity.UserStatus.NORMAL);
        user.setCreatedAt(LocalDateTime.now());
        user.setUpdatedAt(user.getCreatedAt());
        userService.insertForRpc(user);
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
