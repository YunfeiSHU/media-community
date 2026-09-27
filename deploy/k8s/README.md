# Kubernetes 基础设施

这套清单面向已安装 Traefik 的 Kubernetes 集群，提供以下链路：

```text
Internet -> Traefik IngressRoute -> auth-service Service -> Envoy :8081 -> Spring Boot :8002
                                             |
                                             `-> user-service Service -> User Pod 的 Envoy :8081
```

> 本目录从 `auth-service/infra/k8s/` 移到了 `deploy/k8s/`：
> Kubernetes 清单属于**部署资产**，与 compose / 环境变量 / DDL 基线放在一起，
> 不再散落在单个服务目录里。

## 部署

1. 构建镜像（在 `auth-service` 目录下执行，构建上下文是服务目录）：

   ```powershell
   cd auth-service
   docker build -t auth-service:local .
   ```

2. 将镜像导入本地集群。以 kind 为例：

   ```powershell
   kind load docker-image auth-service:local
   ```

   Minikube 可使用 `minikube image load auth-service:local`。

3. 复制并修改两个 Secret 示例中的密码，确保 `MYSQL_PASSWORD` 与 `SPRING_DATASOURCE_PASSWORD` 相同，并替换 JWT secret：

   ```powershell
   kubectl apply -f deploy/k8s/base/namespace.yaml
   kubectl apply -f deploy/k8s/base/auth-secrets.example.yaml
   kubectl apply -f deploy/k8s/base/dependencies.yaml
   kubectl apply -k deploy/k8s/base
   ```

   `auth-secrets.example.yaml` 和 `dependencies.yaml` 中的占位密码只用于演示，生产环境应使用外部 Secret 管理方案。

4. 配置本地解析后访问：

   ```text
   127.0.0.1 auth.localhost
   ```

   然后请求 `http://auth.localhost/api/v1/auth/login`。Traefik 必须已经安装并启用 `traefik.io/v1alpha1` CRD，且存在 `web` entryPoint。

## 设计说明

- `auth-service` Service 只暴露 Envoy 的 8081，Spring Boot 的 8002 不对 Service 直接暴露。
- Auth Deployment 默认 2 个副本；Service 负责 Pod 发现和负载分发。
- Envoy 入站配置包含超时、5xx/连接失败重试、访问日志和 Prometheus 管理端点。
- `user-service` 目前只有 Service，因为本仓库没有 User Spring Boot 实现。User 服务上线后应使用标签 `app.kubernetes.io/name: user-service`，并让它的 Envoy 入站端口为 8081。
- Auth 应用当前没有调用 User 服务的业务代码。Envoy 的 outbound listener 在 `127.0.0.1:15001` 上提供服务间路由；后续 HTTP 客户端需要将 User 请求指向该 listener 并保留 `Host: user-service`，或者再接入透明流量劫持/Istio 类控制面。
- MySQL 和 Redis 清单使用 `emptyDir`，仅适合本地演示。生产环境应替换为 StatefulSet/PVC 或托管数据库，并补充 NetworkPolicy、TLS/mTLS、备份和 Secret 管理。
