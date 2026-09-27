# syntax=docker/dockerfile:1
#
# danmaku-service-go 镜像（弹幕服务，端口 9003，HTTP + WebSocket）
# 数据层与 Java 体系共用（MySQL 3306 / auth_service、Redis 6379 db0）。
# 注意：nginx / 网关转发 WebSocket 时必须带 Upgrade / Connection 头，
# 否则握手会退化成普通 HTTP 请求。

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

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app

ENV TZ=Asia/Shanghai
WORKDIR /app

COPY --from=builder /out/server /app/server
COPY configs /app/configs

USER app
EXPOSE 9003

HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
    CMD wget -qO- http://127.0.0.1:9003/healthz || exit 1

ENTRYPOINT ["/app/server", "-config", "/app/configs/config.yaml"]
