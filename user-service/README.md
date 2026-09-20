# user-service

基于 Spring Boot 3、MyBatis 和 MySQL 的用户资料服务。密码、登录、Token 和认证均由 auth-service 负责。

## 运行

```powershell
mvn spring-boot:run
```

默认地址：`http://localhost:8080`。默认连接本机 MySQL 的 `user_service` 数据库，并初始化一个 `demo` 用户。可通过 `MYSQL_URL`、`MYSQL_USERNAME`、`MYSQL_PASSWORD` 覆盖连接配置。

创建数据库：

```sql
CREATE DATABASE user_service DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

## HTTP 接口

- `GET /api/v1/users/me`：读取当前用户，需要网关传递 `X-User-Id`
- `PUT /api/v1/users/me`：修改当前用户资料，需要 `X-User-Id`
- `DELETE /api/v1/users/me`：逻辑注销，设置为 `DELETED`
- `GET /api/v1/admin/users/{id}`：管理员查询用户，需要 `X-User-Role: ADMIN`
- `PATCH /api/v1/admin/users/{id}/status`：管理员修改状态，需要 `X-User-Role: ADMIN`

## Dubbo 接口

服务接口：`com.example.userservice.rpc.UserDubboService`，Dubbo 协议默认端口 `20880`。

- `createUser`
- `getUser`
- `batchGetUsers`
- `checkUserExists`

Provider 实现见 `src/main/java/com/example/userservice/rpc/UserDubboServiceImpl.java`。本地使用直连模式，生产环境建议接入 Nacos 或 Zookeeper 注册中心。

`X-User-Id` 和 `X-User-Role` 是网关认证后的内部请求头，生产环境应由可信网关覆盖，不能直接信任外部客户端传入。
