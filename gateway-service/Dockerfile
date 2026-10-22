# syntax=docker/dockerfile:1
#
# gateway-service 镜像（API 网关，端口 8000）
# 数据层与 Go 体系共用同一个 MySQL（auth_service 库）与 Redis（db 0），
# 服务注册走 Nacos。镜像里不含任何中间件，运行时靠环境变量指过去
# （见 deploy/compose.java.yml）。

# ---------------------------------------------------------------- 构建阶段
FROM maven:3.9.9-eclipse-temurin-17 AS build

WORKDIR /workspace

# 先只拷依赖清单：只要依赖不变，改业务代码不会重新下载模块（利用层缓存）
COPY pom.xml .
RUN mvn -q -DskipTests dependency:go-offline

COPY src ./src
RUN mvn -q -DskipTests package

# ---------------------------------------------------------------- 运行阶段
FROM eclipse-temurin:17-jre

WORKDIR /app
RUN useradd --system --uid 10001 appuser
COPY --from=build /workspace/target/gateway-service-*.jar /app/gateway-service.jar
USER 10001
EXPOSE 8000

# 就绪判断用 bash 的 /dev/tcp 探测端口：
# 官方 JRE 镜像里没有 curl/wget，这样写五个 Java 镜像可以共用同一套探针，
# 也不必为了探针去给服务加 actuator 依赖。
HEALTHCHECK --interval=30s --timeout=3s --start-period=60s --retries=3 \
    CMD bash -c 'exec 3<>/dev/tcp/127.0.0.1/8000'

ENTRYPOINT ["java", "-jar", "/app/gateway-service.jar"]
