package com.example.userservice.rpc;

import java.io.Serializable;
import java.util.List;
import java.util.Map;

public interface UserDubboService extends Serializable {
    RpcUser createUser(CreateUserCommand command);
    RpcUser getUser(String uid);
    List<RpcUser> batchGetUsers(List<String> uids);
    Map<String, Boolean> checkUserExists(List<String> uids);
}