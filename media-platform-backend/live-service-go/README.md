# live-service-go · 直播服务（业务控制面）

直播间管理 → 开播 → SRS 推流 → 观众拉流 → 在线人数 → 关播。

> **本服务不处理音视频数据**：媒体传输由 SRS 承担；本服务决定「谁能推流」「何时算开播」「观众看哪个地址」，并统计在线人数。

| 项 | 值 |
| --- | --- |
| 端口 | 9002 |
| 数据库 | 与 Java 共用 `auth_service` 库（表：`media_live_room` / `media_live_record`，靠 `media_` 前缀隔离） |
| Redis | 与 Java 共用 `127.0.0.1:6379` db 0（key 前缀 `live:`） |
| 依赖 | MySQL · Redis · RabbitMQ · SRS（可选，`media.driver=mock` 时不需要） |
| 技术栈 | Go · Gin · GORM · go-redis · amqp091-go |

## 文档

| 文档 | 内容 |
| --- | --- |
| [设计文档](docs/设计文档.md) | 与 SRS 分工、状态机与回调对齐、Redis/事件设计、媒体适配器、配置、验收 |
| [接口文档](docs/接口文档.md) | 客户端接口 + **SRS 回调接口**契约、curl 示例、端到端验收脚本 |

## 快速开始

服务一览、端口、仓库级快速开始与目录结构见 `../README.md` §2、§4、§6；本服务只需：

```powershell
go run ./cmd/server
curl.exe http://localhost:9002/readyz
```

需要真实推流时叠加 SRS：`docker compose -f ../deploy/compose.go.yml --profile media up -d`。

## 关键开关

| 环境变量 | 默认 | 说明 |
| --- | --- | --- |
| `LIVE_MYSQL_DSN` | `root:...@tcp(127.0.0.1:3306)/auth_service...` | 与 Java 同库（共库，靠 `media_` 表前缀隔离） |
| `LIVE_REDIS_ADDR` | `127.0.0.1:6379` | 与 Java 同实例、同 db 0（靠 `live:` key 前缀隔离） |
| `LIVE_MEDIA_DRIVER` | `mock` | `mock` 无需 SRS（用 `/dev/publish` 模拟回调）；`srs` 调真实 SRS HTTP API |
| `LIVE_EVENTS_DRIVER` | `rabbitmq` | MQ 不可用时自动降级为进程内总线（仅开发） |

完整配置项见 [设计文档 §11](docs/设计文档.md)。

## 状态机（关键设计）

```text
开播接口：IDLE → STARTING     ← 只表示「已允许推流」，不代表「已在播」
    ↓ 主播用 pushUrl 推流
SRS on_publish：STARTING → LIVING   ← 这才是事实上的开播
    ↓ 主播断流 / 关播接口 / force-close
ENDED（或 STARTING 超时 60s → FAILED）
```

**为什么不直接置 LIVING**：否则主播推流失败（OBS 崩了 / 密钥打错 / 网络不通）时，数据库显示「直播中」但观众看到黑屏，无法区分「服务端问题」与「主播侧问题」。

## 本地无 SRS 时如何联调

创建并开播拿到 `streamKey` 与 `sessionId` 后，用调试接口模拟 SRS 回调（仅 `APP_ENV=local/dev/debug` 时注册）：

```powershell
curl.exe -X POST "http://localhost:9002/api/v1/live/dev/publish/{streamKey}?session={sessionId}"
```

## 已知限制

- 与 video / danmaku 之间只有 RabbitMQ 事件，**没有 gRPC 或任何同步 RPC**；网关直连本服务，未接 Nacos、无负载均衡。
- **没有接入 OpenTelemetry / Jaeger / Prometheus 应用指标**；链路追踪与业务指标均为规划中、未实现。
