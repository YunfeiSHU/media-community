# syntax=docker/dockerfile:1
#
# video-service-go 镜像（点播服务，端口 9001）
# 注意：数据层与 Java 体系共用（MySQL 3306 / auth_service、Redis 6379 db0），
# 因此本镜像**不需要**内置数据库；docker-compose 里通过
# extra_hosts: host.docker.internal 回连宿主机的 Java MySQL/Redis。

# ---------------------------------------------------------------- 构建阶段
FROM golang:1.25-bookworm AS builder

# 国内网络下换用模块代理；离线构建可传 --build-arg GOPROXY=off
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOFLAGS=-mod=mod

WORKDIR /src

# 先只拷依赖清单：只要依赖不变，改业务代码不会重新下载模块（利用层缓存）
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# -trimpath 去掉本机绝对路径；-s -w 去掉符号表与调试信息（镜像更小）
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---------------------------------------------------------------- 运行阶段
FROM alpine:3.20

# tzdata 是必需的：DSN 带 loc=Local，缺时区库会让时间按 UTC 落库
# ffmpeg 也是必需的：VIDEO_TRANSCODER_DRIVER=real 时要调用它转码；
# 不装的话容器里只有 mock 可用，真实上传会一直卡在转码失败。
RUN apk add --no-cache ca-certificates tzdata ffmpeg \
    && adduser -D -u 10001 app

ENV TZ=Asia/Shanghai
WORKDIR /app

COPY --from=builder /out/server /app/server
COPY configs /app/configs

USER app
EXPOSE 9001

HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
    CMD wget -qO- http://127.0.0.1:9001/healthz || exit 1

ENTRYPOINT ["/app/server", "-config", "/app/configs/config.yaml"]
