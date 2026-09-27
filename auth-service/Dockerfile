# syntax=docker/dockerfile:1
#
# auth-service 镜像（鉴权服务，端口 8002）
# 数据层与 Go 体系共用同一个 MySQL（auth_service 库）与 Redis（db 0），
# 服务注册走 Nacos。镜像里不含任何中间件，运行时靠环境变量指过去
# （见 deploy/compose.java.yml）。
#
# 端口说明：本服务 application.yml 的默认端口是 ${SERVER_PORT:8002}，
# 历史上这里写的是 EXPOSE 8080、deploy/k8s 那套清单也按 8080 配，属于不一致；
# 现已统一到 8002（k8s 侧同步改了 containerPort 与 envoy upstream）。

# ---------------------------------------------------------------- 构建阶段
FROM maven:3.9.9-eclipse-temurin-17 AS build

WORKDIR /workspace
COPY pom.xml .
RUN mvn -q -DskipTests dependency:go-offline
COPY src ./src
RUN mvn -q -DskipTests package

# ---------------------------------------------------------------- 运行阶段
FROM eclipse-temurin:17-jre

WORKDIR /app
RUN useradd --system --uid 10001 appuser
COPY --from=build /workspace/target/auth-service-*.jar /app/auth-service.jar
USER 10001
EXPOSE 8002

# 就绪判断用 bash 的 /dev/tcp 探测端口：
# 官方 JRE 镜像里没有 curl/wget，这样写五个 Java 镜像可以共用同一套探针，
# 也不必为了探针去给服务加 actuator 依赖。
HEALTHCHECK --interval=30s --timeout=3s --start-period=60s --retries=3 \
    CMD bash -c 'exec 3<>/dev/tcp/127.0.0.1/8002'

ENTRYPOINT ["java", "-jar", "/app/auth-service.jar"]
