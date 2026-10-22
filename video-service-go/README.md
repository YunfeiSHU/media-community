# video-service-go · 点播服务

视频上传（小文件一次上传 / 大文件**分片上传 + 断点续传**）→ 异步转码（多码率 HLS，默认 `mock`）→ 发布 → 播放。
点播域的唯一写入口，对外只有 HTTP，服务间协作只走 RabbitMQ 事件（交换机 `go.media.video`）。

| 项 | 值 |
| --- | --- |
| 端口 | **9001**（基础路径 `/api/v1`，探针 `/healthz`、`/readyz`） |
| 库表 | 与 Java **共库** `auth_service`：`media_video` / `media_video_category` / `media_video_tag` / `media_transcode_task` / `media_upload_session` |
| 依赖 | MySQL（3306）· Redis（6379 db 0，key 前缀 `video:`）· RabbitMQ（5672）· MinIO（9000，bucket `media`）· FFmpeg（可选） |

## 快速开始

```powershell
# 中间件：MySQL / Redis 直接用 Java 那套；RabbitMQ + MinIO 用 compose 起（编排见 ../deploy/README.md）
docker compose -f ..\deploy\compose.yml up -d

# 启动（默认配置即连上述地址，无需额外环境变量）
go run ./cmd/server
go run ./cmd/server -config configs/config.yaml

# 探针
curl.exe http://localhost:9001/healthz
curl.exe http://localhost:9001/readyz
```

配置项、`VIDEO_*` 环境变量与默认值见 [设计文档 §11](docs/设计文档.md)。

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
├── pkg/                         平台基础设施（errs / response / identity / logx / trace / store / migrate）
├── docs/                        设计文档 / 接口文档
├── tools/                       端到端验收脚本（video-smoke / upload-chunk-smoke）
└── Dockerfile
```

## 文档

| 文档 | 内容 |
| --- | --- |
| [设计文档](docs/设计文档.md) | 系统定位、分层、数据模型与状态机、核心时序、对象存储、缓存、事件、转码适配器、配置、验收标准 |
| [接口文档](docs/接口文档.md) | 全部 HTTP 接口（方法 / 路径 / 鉴权 / 请求字段 / 成功与失败要点）、分片上传与断点续传、验收脚本 |

> 服务一览、端口总表、快速开始与错误码总表见 [`../../README.md`](../README.md)；项目整体定位与部署见该文档 §1–§4。
