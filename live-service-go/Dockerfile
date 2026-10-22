# syntax=docker/dockerfile:1
#
# live-service-go 镜像（直播服务，端口 9002）
# 数据层与 Java 体系共用（MySQL 3306 / auth_service、Redis 6379 db0）。
# SRS 不在本镜像内：compose 里用单独的 srs 服务（profile: media），
# 本服务通过 LIVE_SRS_API_BASE 调用它的 HTTP API。

# ---------------------------------------------------------------- 构建阶段
FROM golang:1.25-bookworm AS builder

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOFLAGS=-mod=mod

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---------------------------------------------------------------- 运行阶段
FROM alpine:3.20

# curl 用于排查 SRS 的 HTTP API（/api/v1/versions），tzdata 供 loc=Local 使用
RUN apk add --no-cache ca-certificates tzdata curl \
    && adduser -D -u 10001 app

ENV TZ=Asia/Shanghai
WORKDIR /app

COPY --from=builder /out/server /app/server
COPY configs /app/configs

USER app
EXPOSE 9002

HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
    CMD wget -qO- http://127.0.0.1:9002/healthz || exit 1

ENTRYPOINT ["/app/server", "-config", "/app/configs/config.yaml"]
