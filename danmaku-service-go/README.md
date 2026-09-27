# danmaku-service-go · 弹幕服务

弹幕实时广播（WebSocket） + 敏感词过滤 + 限流 + 异步落库 + 历史查询。

> **核心设计：双链路解耦。**
> 实时链路（内存 Hub + Redis 热数据）与持久化链路（RabbitMQ → MySQL）互不影响：
> MySQL / Redis / MQ 任一故障都不会导致「观众看不到弹幕」。

| 项 | 值 |
| --- | --- |
| 端口 | 9003 |
| 数据库 | **与 Java 共用** `auth_service` 库（表：`media_danmaku` / `media_sensitive_word`） |
| Redis | **与 Java 共用** `127.0.0.1:6379` db 0（key 前缀 `danmaku:`） |
| 依赖 | MySQL（与 Java 共用）· Redis（与 Java 共用）· RabbitMQ |
| 技术栈 | Go · Gin · GORM · gorilla/websocket · go-redis · amqp091-go |

---

## 文档

| 文档 | 内容 |
| --- | --- |
| [需求文档](docs/需求文档.md) | 目标、范围、角色权限、功能性/非功能性需求、生命周期、验收标准、风险 |
| [设计文档](docs/设计文档.md) | **WebSocket Hub 并发模型（重点）**、敏感词 Trie、限流 Lua、持久化链路、冷热分离、已知限制 |
| [接口文档](docs/接口文档.md) | HTTP 接口 + **WebSocket 帧协议**、客户端示例、端到端验收脚本 |

## 快速开始

```powershell
# 只需 RabbitMQ；MySQL / Redis 用 Java 那套（3306/auth_service、6379 db0）
docker compose up -d
go run ./cmd/server
curl.exe http://localhost:9003/readyz   # 关注 filter 一项的 words 数量
```

## 关键配置开关

| 环境变量 | 默认 | 说明 |
| --- | --- | --- |
| `DANMAKU_EVENTS_DRIVER` | `rabbitmq` | MQ 不可用时降级为进程内总线；MQ 投递失败会进 Redis 补发队列 |
| `DANMAKU_REDIS_ADDR` | `127.0.0.1:6379` | 与 Java 同实例、同 db 0（靠 `danmaku:` key 前缀隔离） |
| `DANMAKU_MYSQL_DSN` | `...@tcp(127.0.0.1:3306)/auth_service...` | 与 Java 同库（共库，靠 `media_` 表前缀隔离） |

限流阈值、热数据条数、心跳周期都在 `configs/config.yaml` 的 `danmaku` / `ws` / `ratelimit` 段。

## WebSocket 协议速览

```text
ws://localhost:9003/ws/danmaku?roomId=5001&uid=u-2001&nickname=小明
ws://localhost:9003/ws/danmaku?videoId=1001

服务端 → 客户端： welcome | danmaku | system | pong | error
客户端 → 服务端： danmaku | ping
```

```javascript
const ws = new WebSocket("ws://localhost:9003/ws/danmaku?roomId=5001&uid=u-2001");
ws.onmessage = e => {
  const f = JSON.parse(e.data);
  if (f.type === "danmaku") render(f.data);       // 广播
  if (f.type === "system")  console.log(f.data);  // LIVE_STARTED / LIVE_ENDED
};
ws.send(JSON.stringify({ type: "danmaku", data: { content: "好耶", clientMsgId: crypto.randomUUID() } }));
setInterval(() => ws.send(JSON.stringify({ type: "ping", data: {} })), 30000);
```

## 并发模型要点

| 设计 | 目的 |
| --- | --- |
| 每连接 2 个 goroutine（readPump / writePump） | `gorilla/websocket` 不允许并发读写；分离后天然满足约束 |
| 每连接带缓冲 `send chan`，广播用 `select` + `default` 丢弃 | **慢客户端不会阻塞整个房间的广播**（背压隔离） |
| 广播先快照客户端列表再遍历 | 避免长时间持读锁导致注册/注销被饿死 |
| 丢弃数量打点 | 「用户说没收到弹幕」可排查 |
| Room 空闲保留 60s 宽限期 | 观众抖动时不反复重建 Hub 结构 |

## 已知限制（阶段一）

1. **单实例广播**：多实例部署时同房间用户收不到彼此弹幕（阶段三用 Redis Pub/Sub 桥接，`hub.Backend` 已预留接口）。
2. 敏感词不做繁简转换。
3. 热数据 TTL 2 小时，超过后第一页查询回落到 MySQL。
4. 点播弹幕完全信任客户端上报的 `timeMs`。

## 目录结构

```text
danmaku-service-go/
├── cmd/server/main.go
├── configs/config.yaml
├── api/dto/dto.go               对外请求 / 响应契约
├── internal/
│   ├── handler/  service/  repository/  model/  middleware/  config/
│   └── infra/
│       ├── events/              事件总线（rabbitmq / local）
│       ├── cache/               房间最近弹幕 / 开播状态
│       ├── hub/                 ⭐ WebSocket Hub / Room / Client / 协议
│       ├── filter/              ⭐ 敏感词前缀树
│       ├── ratelimit/           ⭐ Redis Lua 双层限流
│       └── router/              路由注册
├── pkg/                         平台基础设施
│   └── errs/ response/ identity/ logx/ trace/ store/ migrate/
├── docs/
└── Dockerfile
```
