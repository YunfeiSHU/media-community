# Go 媒体微服务群 · 公共约定

> 适用范围：`video-service-go`（点播）、`live-service-go`（直播）、`danmaku-service-go`（弹幕）。
> 本文档定义三个服务**必须一致**的部分：目录布局、配置方式、统一响应、错误码、身份传递、
> 中间件、消息拓扑、Redis Key 命名、可观测性埋点。
> 各服务自己的需求 / 设计 / 接口细节见对应目录下的 `需求文档.md`、`设计文档.md`、`接口文档.md`。

---

## 1. 与现有 Java 体系的关系

工作区里已经有一套 Java 微服务（`gateway-service` / `auth-service` / `user-service` /
`product-service` / `order-service`），Go 这套服务**刻意对齐它的对外契约**，而不是另起一套：

| 维度 | 对齐方式 |
| --- | --- |
| 身份传递 | 网关（或本地直连时手工）注入 `X-User-Id` / `X-User-Role`，Go 服务**不解析 JWT** |
| 统一响应 | `{"code":0,"message":"success","data":{...}}`，`code=0` 为成功 |
| 错误码 | 沿用 Java 侧编号规范（见 §4），HTTP 状态码与错误码同源 |
| 参数校验失败 | `400 / 40001`，message 为「字段名 + 原因」 |
| **数据层** | **MySQL / Redis 直接复用 Java 体系那一套**（同一实例、同一个 `auth_service` 库、同一个 Redis db 0），不另起一套；靠**表名前缀 `media_`** 与**Redis key 域前缀**避免冲突（见 §2） |

因此 Go 服务可以直接挂在现有网关后面，只要在网关加三条路由即可，不会污染原有链路。
（**已落地**：三条直连路由 + 方法级白名单 + `/ws/danmaku` 查询参数令牌，见
[gateway-service/技术文档.md §10](../gateway-service/技术文档.md)。
注意 Go 服务**未接入 Nacos**，路由必须写直连 `uri` 而非 `lb://`。）

> 共用数据层的代价与收益：收益是部署成本低、可以直接用 Java 侧已有的账号/JWT 身份；
> 代价是共库后表名与 key 名必须自带命名空间，所以下面 §2 的 `media_` 前缀与
> `video:` / `live:` / `danmaku:` 前缀**不是风格偏好，而是硬约束**。

---

## 2. 仓库与端口

三个服务与 Java 服务**平铺同级**，各自独立 `go.mod`（不引入 go.work，避免影响其他 Go 项目）。

```text
media-community-backend/          # 仓库根（8 个服务在本地各自是独立仓库）
├── README.md                     # 项目总览：整体架构 + 接口测试
├── docs/                         # 跨服务文档
│   ├── 直播，点播，弹幕设计方案.md   # 总体方案
│   ├── Go微服务-README.md         # Go 三服务说明
│   ├── Go微服务-公共约定.md        # 本文档
│   ├── Go微服务-问题解决记录.md     # 踩坑记录与排查清单
│   └── 接口测试文档-商品与订单.md
├── deploy/                       # ★ 部署总入口（Java + Go 两套共用）
│   ├── README.md                 # 部署总入口文档
│   ├── .env.example              # 环境变量示例（复制成 deploy/.env）
│   ├── compose.yml               # 编排总入口（include 下面三个）
│   ├── compose.infra.yml         # 共用中间件            profile: infra
│   ├── compose.go.yml            # Go 三服务 + SRS      默认 + profile: media
│   ├── compose.java.yml          # Java 五服务           profile: java
│   ├── db/
│   │   ├── README.md             # 共用库 auth_service 的全部 DDL 与执行顺序
│   │   ├── go/media-tables.sql   # Go 侧建表 DDL（写在 auth_service 库，表名 media_ 前缀，幂等）
│   │   └── verify-schema.ps1     # 校验该 DDL 与 AutoMigrate 实际产出是否完全一致
│   ├── media/srs.conf            # SRS 直播媒体服务器配置
│   └── k8s/                      # Kubernetes 清单
├── tools/publish-all.ps1         # 聚合快照发布脚本
├── gateway-service/              # Java 网关
├── auth-service/                 # Java 鉴权
├── user-service/                 # Java 用户
├── product-service/              # Java 商品
├── order-service/                # Java 订单
├── video-service-go/             # 点播服务
├── live-service-go/              # 直播服务
└── danmaku-service-go/           # 弹幕服务
```

### 端口分配

| 服务 | 端口 | 说明 |
| --- | --- | --- |
| video-service-go | **9001** | HTTP + 内部 gRPC（阶段三） |
| live-service-go | **9002** | HTTP，同时接收 SRS 回调 |
| danmaku-service-go | **9003** | HTTP + WebSocket |
| MySQL | 3306 | **与 Java 共用**（`auth_service` 库），不另起实例 |
| Redis | 6379 | **与 Java 共用**（db 0），不另起实例 |
| RabbitMQ | 5672 / 15672 | Go 体系独有（Java 没有 MQ），沿用官方端口 |
| MinIO | 9000（API） / **9090**（控制台） | Go 体系独有；控制台**不能用 9001**，那个端口被 video-service 占了 |
| SRS | 1936(RTMP) / 8088(HLS) / 1986(API) | profile `media` |

> 只有 **HTTP 端口（9001/9002/9003）需要避开 Java 的 8000/8003/8004**；
> 数据层不再做端口偏移，因为它们是**同一个实例**。
>
> 如果本机没有 Java 那套 MySQL/Redis，可以用 `docker compose --profile infra up -d`
> 起一套独立实例（3307 / 6380），并把 `.env` 里的 `INFRA_*` 改成服务名版本
> （模板见 `deploy/.env.example`）。

### 数据库（与 Java 共库）

三个 Go 服务**全部使用 `auth_service` 库**，与 Java 侧同库。
为在共库场景下表明归属、避免同名冲突，Go 侧表名统一加 **`media_` 前缀**：

| 服务 | 库 | 表（全部带 `media_` 前缀） |
| --- | --- | --- |
| video-service-go | `auth_service` | `media_video`、`media_video_category`、`media_video_tag`、`media_transcode_task` |
| live-service-go | `auth_service` | `media_live_room`、`media_live_record` |
| danmaku-service-go | `auth_service` | `media_danmaku`、`media_sensitive_word` |

Java 侧已占用的表（**Go 侧不得使用这些名字**）：
`account`（auth）、`user`（user）、`product` / `sku` / `inventory`（product）、
`order` / `payment_record`（order）。

建表方式：服务启动时 GORM `AutoMigrate` 自动创建（只增不删）；
也可以用 `deploy/db/go/media-tables.sql` 手工预建（幂等，可重复执行）。
`AutoMigrate` 只处理上面注册的那几张表，**不会触碰 Java 的表**。

> 库里的表不止 Go 这 8 张：Java 侧还有 9 张（`account` / `user` / `product` / `sku` /
> `inventory` / `order` / `order_item` / `payment_order` / `payment_record`）。
> 完整的归属与建表顺序见 **`deploy/db/README.md`**。

### Redis Key 前缀

统一用 `<域>:<实体>:<ID>` 形式。因为**与 Java 共用同一个 Redis 实例与 db 0**，
前缀就是唯一的隔离手段：

- Java 已占用：`auth:blacklist:*`（JWT 黑名单）、`gateway:rate:global:*` / `gateway:rate:ip:*`（网关限流）
- Go 侧占用：`video:*` / `live:*` / `danmaku:*`

**新增 key 必须先确认前缀未被占用**；不要在 Go 侧使用 `auth:` / `gateway:` 开头的 key。

| Key | 类型 | 归属 | 含义 |
| --- | --- | --- | --- |
| `video:detail:{videoId}` | String(JSON) | video | 视频详情缓存 |
| `video:view:{videoId}` | String(计数) | video | 播放量缓冲计数 |
| `live:room:{roomId}` | Hash | live | 直播间实时状态 |
| `live:online:{roomId}` | ZSet | live | 在线观众（score = 心跳时间戳） |
| `live:key:{streamKey}` | String | live | 推流密钥 → roomId 反查 |
| `danmaku:room:{roomId}:recent` | List | danmaku | 最近弹幕（热数据，LPUSH + LTRIM 500） |
| `danmaku:video:{videoId}:recent` | List | danmaku | 点播弹幕热数据 |
| `danmaku:rate:u:{uid}` | String(计数) | danmaku | 用户级限流窗口（1s） |
| `danmaku:rate:room:{roomId}` | String(计数) | danmaku | 房间级限流窗口（1s） |
| `danmaku:live:{roomId}` | String | danmaku | 房间是否直播中（由 `live.*` 事件维护） |
| `danmaku:dedup:{clientMsgId}` | String | danmaku | 客户端重试去重（10s，值为首发响应快照） |
| `danmaku:idem:{eventId}` | String | danmaku | 落库消费幂等 |
| `danmaku:pending:persist` | List | danmaku | MQ 不可用时的补发队列 |

> 各服务**完整**的 key 清单以各自的《设计文档》为准（上表只列跨服务的关键域）。
> `mq:idem:{eventId}` 是三个服务共用的消费幂等 key 前缀（见 §7）。
>
> 扫描类操作（如 `video:view:*` 回写播放量）必须**带完整域前缀**，
> 绝不能对共用的 db 0 做 `KEYS *` / `FLUSHDB`。

---

## 3. 统一目录结构

三个服务使用同一套骨架，保证「换个服务也能立刻读懂」：

```text
<svc>/
├── cmd/server/main.go              # 进程入口：装配依赖 → 启动 → 优雅退出
├── configs/config.yaml             # 默认配置（支持 ${ENV:-default} 占位）
├── api/dto/dto.go                  # 对外请求 / 响应契约
├── internal/                       # 本服务实现，禁止被外部导入
│   ├── handler/                    # HTTP 出入口（只做参数绑定 + 调 service）
│   ├── service/                    # 业务逻辑
│   ├── repository/                 # 数据访问
│   ├── model/                      # GORM 实体
│   ├── middleware/                 # RequestID / Recovery / Logger / CORS / Auth
│   ├── config/config.go            # 配置加载（YAML + 环境变量覆盖）
│   └── infra/                      # 领域基础设施适配器（依赖本服务 model / dto）
│       ├── events/                 # bus.go / local.go / rabbit.go（事件总线）
│       ├── cache/                  # Redis 缓存
│       ├── router/router.go        # 路由注册
│       └── media/  storage/        # 按服务需要出现
├── pkg/                            # 平台基础设施（与业务无关，可复用）
│   ├── errs/errs.go                # ErrorCode 枚举 + BizError
│   ├── response/response.go        # 统一响应体与渲染函数
│   ├── identity/identity.go        # CurrentUser + 归属判断 + 请求头解析（X-User-Id / X-User-Role）
│   ├── logx/  trace/               # 日志 / 链路追踪
│   └── store/  migrate/            # MySQL / Redis 连接封装、AutoMigrate
├── docs/                           # 需求文档.md / 设计文档.md / 接口文档.md
├── Dockerfile                      # 多阶段构建（docker-compose 的 build.context 指向服务目录）
├── Makefile                        # run / build / test / vet / fmt / docker
└── README.md
```

> 建表 DDL 不在服务目录内，而是仓库 `deploy/db/go/media-tables.sql`
> （三个服务共库，见 §2）。

> `danmaku-service-go` 在本骨架之上多三个自有包（其余完全同构）：
> `internal/infra/hub`（WebSocket 连接与房间广播）、`internal/infra/filter`（敏感词前缀树）、
> `internal/infra/ratelimit`（Redis Lua 双层限流）。
> `live-service-go` 的 `internal/infra/media` 是 SRS 适配器（mock / srs 两种驱动）；
> `video-service-go` 的 `internal/infra/media`（转码）与 `internal/infra/storage`（MinIO）同理。

**分层纪律**：`handler` 不写业务、不碰 GORM；`service` 不感知 `*gin.Context`；
`repository` 只做数据存取；三者之间用 `context.Context` + 实体/领域对象传递。

---

## 4. 错误码规范

沿用 Java 侧编号（见同目录 [`接口测试文档-商品与订单.md`](接口测试文档-商品与订单.md) §1.6），并补充媒体业务专用段：

| code | HTTP | 含义 | 使用范围 |
| --- | --- | --- | --- |
| 0 | 200 | 成功 | 全部 |
| 40001 | 400 | 请求参数不合法 | 全部 |
| 40101 | 401 | 未认证（缺 `X-User-Id`） | 全部 |
| 40301 | 403 | 无权限 / 非资源所有者 | 全部 |
| 40401 | 404 | 资源不存在（服务内自定义后两位） | 全部 |
| 40901 | 409 | 状态冲突（如重复开播、重复点赞） | 全部 |
| 42901 | 429 | 请求过于频繁 | danmaku / live |
| 50000 | 500 | 内部错误 | 全部 |
| 50001 | 503 | 下游依赖不可用（MySQL/Redis/MQ/MinIO） | 全部 |
| 60001 | 400 | 文件类型不支持 | video |
| 60002 | 413 | 文件超出大小限制 | video |
| 60003 | 409 | 视频状态不允许该操作 | video |
| 60004 | 503 | 转码任务提交失败 | video |
| 70001 | 409 | 直播间不在直播中 | live |
| 70002 | 409 | 推流密钥无效或会话已过期 | live |
| 70003 | 409 | 超出单用户直播间数量上限 | live |
| 70004 | 409 | 该用户已有正在直播的直播间 | live |
| 70005 | 404 | 直播间不存在 | live |
| 70006 | 503 | 媒体服务器（SRS）不可用 | live |
| 80001 | 400 | 弹幕内容不合法（空 / 超长 / 命中敏感词） | danmaku |
| 80002 | 429 | 弹幕发送过于频繁（`data.retryAfterMs`） | danmaku |
| 80003 | 409 | 直播间当前未开播，不允许发送弹幕 | danmaku |

> `40401` / `40901` 这类「通用编号」在各服务内按资源细分，例如弹幕撤回不存在的 ID 是
> `40401 弹幕不存在`，敏感词重复是 `40901 敏感词已存在: <词>`。编号保持不变，
> 具体含义由各服务的《接口文档》定义。

实现方式：`pkg/errs/errs.go` 定义 `ErrorCode{Code int, Message string, HTTPStatus int}` 与
`BizError`；`handler` 只返回 `errs.New(errs.CodeXXX, ...)`，由统一错误中间件渲染成响应体。

---

## 5. 统一响应体

```json
{ "code": 0, "message": "success", "data": { } }
```

与 Java 侧差异（**刻意为之**，更贴近 Go 习惯且不破坏契约）：

- 失败时 `data` 为 `null`（Java 侧 `non_null` 会省略该字段）。前端只需判断 `code`，两种写法都能兼容。
- 时间统一 `RFC3339`（`2026-09-27T10:30:00+08:00`），时区 `Asia/Shanghai`。
- 长整型 ID 在 JSON 中按数字输出（Go `int64`），前端应使用 `Number` 安全范围内的 ID 或按字符串兜底处理。

---

## 6. 身份与中间件约定

| 中间件 | 行为 |
| --- | --- |
| `RequestID` | 读 `X-Request-Id`，缺失则生成 UUID，回写响应头并注入 `context` |
| `Recovery` | 捕获 panic → 日志 + `500/50000` |
| `Logger` | 记录 method / path / status / 耗时 / requestId / uid |
| `CORS` | 允许跨域，用于 HLS 播放与 WebSocket 握手 |
| `Auth` | 解析 `X-User-Id` / `X-User-Role` 到 `context`，缺失即 `401 / 40101` |
| `AdminOnly` | 要求 `X-User-Role: ADMIN`，否则 `401` / `403`（挂在 `Auth` 之后） |

- uid 唯一来源 `X-User-Id`；角色唯一来源 `X-User-Role`（缺失按 `user`）。
- 需要登录的接口缺 `X-User-Id` → `401 / 40101`；路由上用 `middleware.Auth()` 表达。
- 资源写操作校验归属：`X-User-Id != 资源.ownerId` → `403 / 40301`（service 层判断）。
- 「登录可有可无」的接口（如直播间观众心跳）**不挂 `Auth`**，由 handler 自行判定；
  白名单接口（视频详情、直播间列表、弹幕历史、SRS 回调）同样不挂。

---

## 7. 消息拓扑（RabbitMQ）

统一使用 **topic exchange + 持久化队列 + 手动 ACK + 死信队列**。

| Exchange | Routing Key | 生产者 | 消费者 | 用途 |
| --- | --- | --- | --- | --- |
| `go.media.video` | `video.uploaded` | video-service | video-service（transcode worker） | 触发异步转码 |
| `go.media.video` | `video.transcoded` | video-service | video-service | 转码完成回调（发布/通知） |
| `go.media.live` | `live.started` | live-service | live-service（统计）、**danmaku-service（房间状态 + WS system 帧）** | 开播事件 |
| `go.media.live` | `live.stopped` | live-service | live-service、**danmaku-service** | 关播事件 |
| `go.media.danmaku` | `danmaku.created` | danmaku-service | danmaku-service（persist worker） | 弹幕异步落库 |

### 7.1 命名规则（两个容易踩的坑，都是实测踩过的）

**坑一：队列名必须带「消费方」前缀，不能只用 routing key。**

`live.started` 同时被 live-service 和 danmaku-service 订阅。若队列名只由 routing key 推导
（`live.started.q`），两个服务会争抢同一个队列 —— 而且各自的 DLX 是按自己那个 exchange
命名的，后启动的服务会直接拿到
`406 PRECONDITION_FAILED - inequivalent arg 'x-dead-letter-exchange'` 而**启动失败**。

```
队列   = <消费方 exchange>.<routingKey>.q     # go.media.danmaku.live.started.q
DLQ    = <消费方 exchange>.<routingKey>.dlq   # go.media.danmaku.live.started.dlq
DLX    = <消费方 exchange>.dlx                # go.media.danmaku.dlx
```

**坑二：订阅要绑到「生产方」的交换机，不是自己那个。**

路由键形如 `<域>.<事件>`，域就是生产方的 exchange 后缀（`events.ExchangeFor(routingKey)`）。
danmaku 订阅 `live.started`，但 live-service 是发到 `go.media.live` 的；如果绑到自己的
`go.media.danmaku`，两边就像活在平行世界 —— **队列有消费者、绑定也显示成功、没有任何报错，
只是永远收不到消息**。这类问题只能靠端到端联调发现，单元测试永远测不出来。

```
go.media.live → go.media.danmaku.live.started.q   (routing_key=live.started)   ✅
go.media.danmaku → go.media.danmaku.live.started.q                            ❌
```

### 7.2 载荷字段类型约定

**约定**：所有消息体为 JSON，外层信封固定为 `eventId`（幂等键）、`eventType`、`occurredAt`、
`traceId`、`payload`。消费者按 `eventId` 在 Redis 做短时幂等去重（`SETNX mq:idem:{eventId}`）。

**`roomId` 在事件载荷里一律是字符串**（`"roomId":"5"`，不是 `5`）。房间在 danmaku / 网关 /
Redis key 里都是字符串形态；发布方若图省事直接塞 `uint64`，消费方会得到

```
json: cannot unmarshal number into Go struct field LiveStartedPayload.roomId of type string
```

然后消息一直重试到进死信队列，业务侧表现为「功能静默失效」。发布侧用
`strconv.FormatUint(id, 10)` 转一次即可。其他 ID 字段（`videoId`、`uid`、`sessionId`）同理。

> 本节只写**必须遵守的规则**。上述三个坑的完整排查过程（含 `406` 报文、
> `/api/bindings` 定位手法、死信深度查看命令）见 **`Go微服务-问题解决记录.md` §1**。

---

## 8. 配置加载约定

`configs/config.yaml` 支持 `${VAR:-default}` 占位，由 `internal/config` 在解析前展开：

```yaml
mysql:
  # 与 Java 共用：同一个 MySQL 实例、同一个 auth_service 库
  dsn: "${VIDEO_MYSQL_DSN:-root:***REMOVED***@tcp(127.0.0.1:3306)/auth_service?charset=utf8mb4&parseTime=True&loc=Local}"
redis:
  # 与 Java 共用：同一个 Redis 实例、同一个 db 0
  addr: "${VIDEO_REDIS_ADDR:-127.0.0.1:6379}"
  password: "${VIDEO_REDIS_PASSWORD:-}"
  db: 0
```

- 环境变量命名：`<SERVICE>_<SECTION>_<KEY>`（如 `VIDEO_MINIO_ENDPOINT`）。
- 配置结构体字段用 `yaml` tag（与文件键名一一对应，不引入 viper / mapstructure）。
- `APP_ENV` 控制 gin mode、日志格式与「开发专用路由是否注册」（如 live 的 dev 推流回调）。
- 生产配置只用环境变量覆盖，**不把密钥写进仓库**。

**数值型占位符不能加引号。** 默认值是在 YAML 解析**之前**做字符串替换的，
所以 `rtmp_port: "${LIVE_SRS_RTMP_PORT:-1936}"` 展开后是带引号的字符串 `"1936"`，
yaml.v3 无法写进 `int` 字段，服务**直接起不来**：

```
yaml: unmarshal errors: line 47: cannot unmarshal !!str '1936' into int
```

正确写法是去掉引号（`rtmp_port: ${LIVE_SRS_RTMP_PORT:-1936}`），只有字符串型才加引号。
详见 `Go微服务-问题解决记录.md` §2.1。

---

## 9. 分阶段落地路线

三个服务共用同一份阶段划分（每个服务的《设计文档》里只展开与自身相关的部分）：

| 阶段 | 目标 | 关键交付 |
| --- | --- | --- |
| **阶段一（本次交付）** | 可编译可跑的骨架 + 核心业务闭环 | 配置、统一响应/错误码、中间件、AutoMigrate、核心 CRUD、健康检查、docker-compose |
| 阶段二 | 异步化与媒体处理 | RabbitMQ 事件、转码 Worker、FFmpeg 适配器、弹幕异步落库、在线人数心跳清理 |
| 阶段三 | 真实直播媒体链路 | SRS 接入、推流鉴权回调、录制回放、WebRTC/HTTP-FLV |
| 阶段四 | 生产化与可观测性 | OpenTelemetry + Jaeger、Prometheus 指标、限流熔断、K8s 部署清单 |

**阶段一完成判据**：`go build ./...` 通过、`/healthz` 与 `/readyz` 可用、核心接口能用 curl 走通、
`docker compose up -d` 后三个服务均能连上 MySQL/Redis/RabbitMQ。

---

## 10. 快速开始

```powershell
# 1) 准备环境变量（可选，不设置也能跑：默认值就是「连 Java 那套 MySQL/Redis」）
Copy-Item deploy\.env.example deploy\.env

# 2) 启动 Go 体系独有组件：RabbitMQ + MinIO
#    MySQL / Redis 用 Java 那套（127.0.0.1:3306 / 127.0.0.1:6379），不需要 docker
#    ★ 本机已手工跑着 rabbitmq / minio / nacos，所以**不加** --profile infra
#    如果什么都没有，改用：cd deploy; docker compose --profile infra up -d
#    （会把 MySQL/Redis 起在 3307/6380，记得把 .env 里的 INFRA_* 改成服务名版本）
cd deploy
docker compose up -d

# 3) 建表：可手工执行 DDL，也可以直接启动服务由 AutoMigrate 自动建表
#    注意 --default-character-set=utf8mb4：Windows 上 mysql 客户端默认按本地代码页（中文系统是 GBK）
#    解码 SQL 文件，文件里 UTF-8 的中文注释会被误配对并吞掉后面的引号，
#    报出看起来毫不相关的 `ERROR 1064 ... near 'DRAFT'`。
mysql -h127.0.0.1 -uroot -p --default-character-set=utf8mb4 auth_service < deploy/db/go/media-tables.sql
#    （DDL 里是 CREATE TABLE IF NOT EXISTS，所以对着已经有表的库执行等于空转，不会报错也不会改结构）

# 3.1) 可选：校验 DDL 文件与「AutoMigrate 实际建出来的结构」是否一致
#      （它会在临时库里真跑一遍 DDL，再逐表比对列与索引，最后删掉临时库）
powershell -NoProfile -ExecutionPolicy Bypass -File deploy/db/verify-schema.ps1

# 4) 分别启动三个服务（各自目录）
#    默认就连 127.0.0.1:3306/auth_service 与 127.0.0.1:6379 db0，无需额外配置
cd video-service-go;   go run ./cmd/server
cd live-service-go;    go run ./cmd/server
cd danmaku-service-go; go run ./cmd/server

# 5) 健康检查
curl.exe http://localhost:9001/healthz   # video
curl.exe http://localhost:9002/healthz   # live
curl.exe http://localhost:9003/healthz   # danmaku
```

管理台：

- RabbitMQ：<http://localhost:15672>（`admin` / `admin`）
- MinIO：<http://localhost:9090>（`minioadmin` / `minioadmin`）
- MySQL / Redis：与 Java 体系共用同一实例，用你现有的客户端连即可（库 `auth_service`）
