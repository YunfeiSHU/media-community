# video-service-go · 点播服务

视频上传（小文件一次上传 / 大文件**分片上传 + 断点续传**）→ 异步转码（FFmpeg 多码率 HLS）→ 发布 → 播放。

| 项 | 值 |
| --- | --- |
| 端口 | 9001 |
| 数据库 | **与 Java 共用** `auth_service` 库（表：`media_video` / `media_video_category` / `media_video_tag` / `media_transcode_task` / `media_upload_session`） |
| Redis | **与 Java 共用** `127.0.0.1:6379` db 0（key 前缀 `video:`） |
| 依赖 | MySQL（与 Java 共用）· Redis（与 Java 共用）· RabbitMQ · MinIO · FFmpeg（可选） |
| 技术栈 | Go · Gin · GORM · go-redis · amqp091-go · minio-go |

---

## 文档

| 文档 | 内容 |
| --- | --- |
| [需求文档](docs/需求文档.md) | 目标、范围、角色权限、功能性/非功能性需求、状态机、验收标准、阶段划分、风险 |
| [设计文档](docs/设计文档.md) | 系统定位、分层、数据模型与 DDL、核心时序、对象存储、缓存、事件、转码适配器、配置 |
| [接口文档](docs/接口文档.md) | 全部 HTTP 接口的请求/响应/错误码、curl 示例、端到端验收脚本 |

> 表名带 `media_` 前缀是**硬约束**：本服务与 Java 的 auth/user/product/order **共用一个库**，
> 无前缀的 `video` 这类名字既无法表达归属，也容易和 Java 侧未来的表撞名。

## 快速开始

```powershell
# 基础设施：只需 RabbitMQ + MinIO（中间件都在宿主机上，见 deploy/README.md）
# MySQL / Redis 直接用 Java 那套（127.0.0.1:3306/auth_service、127.0.0.1:6379 db0）
# 本机什么都没有时：cd deploy; docker compose --profile infra up -d（3307 / 6380）
docker compose -f deploy/compose.yml up -d

# 启动服务（默认即连上述地址，无需额外环境变量）
go run ./cmd/server
# 或指定配置
go run ./cmd/server -config configs/config.yaml

# 健康检查
curl.exe http://localhost:9001/healthz
curl.exe http://localhost:9001/readyz
```

## 关键配置开关

| 环境变量 | 默认 | 说明 |
| --- | --- | --- |
| `VIDEO_MYSQL_DSN` | `...@tcp(127.0.0.1:3306)/auth_service...` | 与 Java 同库（共库，靠 `media_` 表前缀隔离） |
| `VIDEO_REDIS_ADDR` | `127.0.0.1:6379` | 与 Java 同实例、同 db 0（靠 `video:` key 前缀隔离） |
| `VIDEO_TRANSCODER_DRIVER` | `mock` | `mock` 生成占位 HLS（无需 ffmpeg）；`real` 调用本机 ffmpeg 产出真实多码率 HLS |
| `VIDEO_EVENTS_DRIVER` | `rabbitmq` | MQ 不可用时自动降级为进程内事件总线（仅开发） |
| `VIDEO_FFMPEG_PATH` | `ffmpeg` | ffmpeg 可执行文件路径（Windows 建议写绝对路径） |
| `VIDEO_MINIO_PUBLIC_ENDPOINT` | `http://127.0.0.1:9000` | 返回给前端的播放地址前缀（与容器内网地址区分） |

## 核心链路

```text
POST /videos                    创建草稿 (DRAFT)
POST /videos/:id/upload         MinIO 一次性上传原片（小文件）→ TRANSCODING → 投递 video.uploaded
POST /videos/:id/upload/init    大文件分片上传：init → parts/:n → complete（可断点续传）
   ↓ (Transcode Worker 消费)
GET  /videos/:id/transcode      查询进度（PENDING/RUNNING/SUCCESS/FAILED）
   ↓ 转码成功，状态 READY
POST /videos/:id/publish        发布 (PUBLISHED)
GET  /videos/:id/play           取 HLS 播放地址 + 多码率 sources
POST /videos/:id/offline        下架 (OFFLINE)
```

### 分片上传与断点续传

大文件走 5 个接口：`init` → `PUT parts/:n`（可并发、可中断）→ `complete`，
另有 `GET status` 查进度与 `DELETE` 取消。

- **切片大小以 `init` 返回的 `partSize` 为准**，非末片必须正好等于它
  （另有 S3 协议下限 5MiB、上限 10000 片）；
- **断点续传的权威进度来自 MinIO `ListParts`**：服务端不另存进度副本，
  `media_upload_session` 表只保存 `uploadId ↔ videoId` 映射与切片计划；
- 同参数重复 `init` 会**复用同一 `uploadId`**，已上传的分片不会失效；
- 与一次性上传在「原片已就位」后走同一条转码编排路径（建任务 → 改状态 → 发事件）。

端到端验收：`pwsh -File tools/upload-chunk-smoke.ps1`（49 项断言）。

## 目录结构

```text
video-service-go/
├── cmd/server/main.go           进程入口（装配 + 优雅退出）
├── configs/config.yaml          默认配置（支持 ${ENV:-default}）
├── api/dto/dto.go               对外请求 / 响应契约
├── internal/
│   ├── handler/  service/  repository/  model/  middleware/  config/
│   └── infra/
│       ├── events/              事件总线（rabbitmq / local）
│       ├── cache/               详情缓存、分类缓存、播放量缓冲
│       ├── media/               转码器（mock / ffmpeg）+ 码率档位
│       ├── storage/             MinIO 封装（含 multipart 分片上传）
│       └── router/              路由注册
├── pkg/                         平台基础设施
│   ├── errs/  response/         错误码 + 统一响应
│   ├── identity/                身份模型 + 请求头解析
│   ├── logx/  trace/            日志 / 链路
│   └── store/  migrate/         MySQL / Redis 连接、AutoMigrate
├── docs/                        需求 / 设计 / 接口文档
└── Dockerfile
```
