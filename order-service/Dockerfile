# syntax=docker/dockerfile:1
#
# order-service 镜像（订单服务，端口 8004；Dubbo 消费端端口随机分配）
# 数据层与 Go 体系共用同一个 MySQL（auth_service 库）。
# 它通过 Dubbo **直连** product-service 的 20880（PRODUCT_RPC_URL），
# 不依赖注册中心，因此 compose 里用服务名 dubbo://product-service:20880。
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
COPY --from=build /workspace/target/order-service-*.jar /app/order-service.jar
USER 10001
EXPOSE 8004

# 就绪判断用 bash 的 /dev/tcp 探测端口：
# 官方 JRE 镜像里没有 curl/wget，这样写五个 Java 镜像可以共用同一套探针，
# 也不必为了探针去给服务加 actuator 依赖。
HEALTHCHECK --interval=30s --timeout=3s --start-period=60s --retries=3 \
    CMD bash -c 'exec 3<>/dev/tcp/127.0.0.1/8004'

ENTRYPOINT ["java", "-jar", "/app/order-service.jar"]
