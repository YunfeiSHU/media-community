# syntax=docker/dockerfile:1
#
# product-service 镜像（商品服务，端口 8003；Dubbo 服务端口 20880）
# 数据层与 Go 体系共用同一个 MySQL（auth_service 库）。
# 注意：它的 resources/db/schema.sql **不会**被 Spring 自动执行，
# 需要在库建好后手工导入一次（见 deploy/db/README.md）。

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
COPY --from=build /workspace/target/product-service-*.jar /app/product-service.jar
USER 10001
# 8003 是 HTTP，20880 是 Dubbo（order-service 会直连它）
EXPOSE 8003 20880

# 就绪判断用 bash 的 /dev/tcp 探测端口：
# 官方 JRE 镜像里没有 curl/wget，这样写五个 Java 镜像可以共用同一套探针，
# 也不必为了探针去给服务加 actuator 依赖。
HEALTHCHECK --interval=30s --timeout=3s --start-period=60s --retries=3 \
    CMD bash -c 'exec 3<>/dev/tcp/127.0.0.1/8003'

ENTRYPOINT ["java", "-jar", "/app/product-service.jar"]
