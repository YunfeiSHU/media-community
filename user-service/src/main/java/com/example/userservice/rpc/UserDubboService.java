package com.example.userservice.rpc;

import java.util.List;
import java.util.Map;

public interface UserDubboService {
    RpcUser createUser(CreateUserCommand command);

    RpcUser getUser(String uid);

    List<RpcUser> batchGetUsers(List<String> uids);

    Map<String, Boolean> checkUserExists(List<String> uids);
}
