# auth-service

基于 Java、Spring Boot、MyBatis 和 MySQL 的登录注册服务。

## 启动前准备

1. 创建数据库：`CREATE DATABASE auth_service DEFAULT CHARACTER SET utf8mb4;`
2. 修改 `src/main/resources/application.yml` 中的 MySQL 用户名和密码。
3. 执行 `src/main/resources/schema.sql` 创建 `user` 表。
4. 使用 JDK 17 和 Maven 运行：`mvn spring-boot:run`

## 接口

### 注册

`POST /api/v1/auth/register`

```json
{
  "username": "yunfei",
  "password": "Abc123456"
}
```

### 登录

`POST /api/v1/auth/login`

```json
{
  "username": "yunfei",
  "password": "Abc123456"
}
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": null
}
```
