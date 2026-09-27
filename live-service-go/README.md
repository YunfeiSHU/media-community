# live-service-go · 直播服务（业务控制面）

直播间管理 → 开播 → SRS 推流 → 观众拉流 → 在线人数 → 关播。

> **本服务不处理音视频数据。** 真正的媒体传输由 SRS 承担；
> 本服务决定「谁能推流」、「何时算开播」、「观众看哪个地址」、以及统计在线人数。

| 项 | 值 |
| --- | --- |
| 端口 | 9002 |
| 数据库 | **与 Java 共用** `auth_service` 库（表：`media_live_room` / `media_live_record`） |
| Redis | **与 Java 共用** `127.0.0.1:6379` db 0（key 前缀 `live:`） |
| 依赖 | MySQL（与 Java 共用）· Redis（与 Java 共用）· RabbitMQ · SRS（可选） |
| 技术栈 | Go · Gin · GORM · go-redis · amqp091-go |

---

## 文档

| 文档 | 内容 |
| --- | --- |
| [需求文档](docs/需求文档.md) | 目标、范围、角色权限、功能性/非功能性需求、状态机、验收标准、风险 |
| [设计文档](docs/设计文档.md) | 与 SRS 的分工、状态机与回调对齐、Redis 设计、事件、媒体服务适配器、配置 |
| [接口文档](docs/接口文档.md) | 客户端接口 + **SRS 回调接口**契约、curl 示例、端到端验收脚本 |

## 快速开始

```powershell
# 基础设施：RabbitMQ + MinIO 必需；SRS 真实推流时才需要（编排在 deploy/）
docker compose -f deploy/compose.yml up -d
# MySQL / Redis 直接用 Java 那套（3306/auth_service、6379 db0）
# 本机什么都没有时：cd deploy; docker compose --profile infra up -d（3307 / 6380）
docker compose -f deploy/compose.yml --profile media up -d  # 额外拉起 SRS

go run ./cmd/server
curl.exe http://localhost:9002/readyz
```

## 关键配置开关

| 环境变量 | 默认 | 说明 |
| --- | --- | --- |
| `LIVE_MYSQL_DSN` | `...@tcp(127.0.0.1:3306)/auth_service...` | 与 Java 同库（共库，靠 `media_` 表前缀隔离） |
| `LIVE_REDIS_ADDR` | `127.0.0.1:6379` | 与 Java 同实例、同 db 0（靠 `live:` key 前缀隔离） |
| `LIVE_MEDIA_DRIVER` | `mock` | `mock` 无需 SRS 即可联调（提供 `/dev/publish` 模拟回调）；`srs` 调真实 SRS HTTP API |
| `LIVE_SRS_RTMP_HOST` | `localhost` | 推流地址中的宿主机名（返回给主播客户端） |
| `LIVE_SRS_RTMP_PORT` | `1936` | RTMP 端口 |
| `LIVE_SRS_HTTP_PORT` | `8088` | HLS / HTTP-FLV 端口 |
| `LIVE_SRS_API_BASE` | `http://127.0.0.1:1986` | SRS HTTP API（用于强制断流） |
| `LIVE_EVENTS_DRIVER` | `rabbitmq` | MQ 不可用时自动降级为进程内总线 |

## 状态机（关键设计）

```text
开播接口：IDLE → STARTING     ← 只表示「已允许推流」，不代表「已在播」
    ↓ 主播用 pushUrl 推流
SRS on_publish：STARTING → LIVING   ← 这才是事实上的开播
    ↓ 主播断流 / 关播接口 / force-close
ENDED（或 STARTING 超时 60s → FAILED）
```

**为什么不直接置 LIVING**：否则主播推流失败（OBS 崩了 / 密钥打错 / 网络不通）时，
数据库显示「直播中」但观众看到黑屏，无法区分「服务端问题」与「主播侧问题」。

## 核心链路

```text
POST /live/rooms                    创建直播间（返回 streamKey / pushUrl）
POST /live/rooms/:id/start          开播 → 生成新密钥 + sessionId → STARTING
POST /live/hooks/on-publish         SRS 回调 → 校验密钥与 session → LIVING → 发 live.started
GET  /live/rooms/:id/play           取 HLS / HTTP-FLV / WebRTC 播放地址
POST /live/rooms/:id/heartbeat      观众心跳（ZSet 维护在线人数）
GET  /live/rooms/:id/online         轻量轮询（只读 Redis）
POST /live/rooms/:id/stop           关播 → 落库直播记录 → 发 live.stopped
```

## 本地无 SRS 时如何联调

```powershell
# 1) 创建并开播，拿到 streamKey 与 sessionId
# 2) 用调试接口模拟 SRS 的 on_publish 回调（仅 APP_ENV=local/dev/debug 时注册）
curl.exe -X POST "http://localhost:9002/api/v1/live/dev/publish/{streamKey}?session={sessionId}"
# 3) 此时房间状态会变为 LIVING，可正常取播放地址与上报心跳
```

## 目录结构

```text
live-service-go/
├── cmd/server/main.go
├── configs/config.yaml
├── api/dto/dto.go               对外请求 / 响应契约
├── internal/
│   ├── handler/  service/  repository/  model/  middleware/  config/
│   └── infra/
│       ├── events/              事件总线（rabbitmq / local）
│       ├── cache/               直播间状态缓存
│       ├── media/               MediaServer 适配器（mock / srs_api）
│       └── router/              路由注册
├── pkg/                         平台基础设施
│   └── errs/ response/ identity/ logx/ trace/ store/ migrate/
├── docs/
└── Dockerfile
```
