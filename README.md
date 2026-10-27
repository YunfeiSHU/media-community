# 音视频社区（media-platform）

B 站风格的音视频社区：**8 个后端服务**（Java 5 + Go 3）+ **React 前端**。
支持点播上传与播放、直播与弹幕、点赞/收藏/关注、商城下单与模拟支付。

> 这是一个面向作品集的项目：重点在**契约一致、边界清晰、问题有记录**，
> 而不是堆功能。所有“为什么这么做”的取舍都写在文档里。

---

## 1. 架构

```text
浏览器
  └─ Nginx（同源入口，唯一公网端口）
       ├─ /            → web（React SPA）
       ├─ /api         → gateway:8000（JWT 校验 → 注入 X-User-Id/X-User-Role）
       ├─ /ws          → gateway → danmaku-service（WebSocket）
       ├─ /media       → MinIO（HLS 分片、封面）
       └─ /live        → SRS（HLS / HTTP-FLV）

gateway ─┬─ auth-service     (8002) 登录 / 刷新 / 登出 / 验证码
         ├─ user-service     (8001) 资料 / 公开 profiles / 关注
         ├─ product-service  (8003) 商品 / SKU / 库存
         ├─ order-service    (8004) 订单 / 支付 / mock-pay
         ├─ video-service-go (9001) 视频 / 分片上传 / 转码 / 图片 / 点赞收藏
         ├─ live-service-go  (9002) 直播间 / 开播 / 心跳 / 在线人数
         └─ danmaku-service-go (9003) 弹幕 REST + WebSocket
```

**关键约定**

- 下游服务只信任网关注入的 `X-User-Id` / `X-User-Role`，不解析 JWT。
- 需要“游客可看、登录后看自己的”接口走 `optional-auth`（有令牌校验注入、无令牌放行）。
- 页面与接口同源，不依赖 CORS；媒体地址一律相对路径。
- 统一响应 `{code,message,data}`；时间目标格式 RFC3339 `+08:00`。

---

## 2. 快速开始

### 前端（无需后端也能看页面）

```bash
cd web
pnpm install
pnpm dev          # http://localhost:5173
```

默认 `VITE_USE_MOCK=1`，用内置 mock 数据渲染首页/播放页/商城。
接真实后端时删掉 `.env.development` 里的这一行即可。

### 后端

一条命令（推荐）：

```powershell
cd media-platform-backend/deploy
.\dev-up.ps1        # 起 infra + 导表 + 建桶 + 启动 8 个服务
.\dev-down.ps1      # 停止（-KeepInfra 只停服务）
```

手工方式：

```bash
cd media-platform-backend/deploy
cp .env.example .env      # 填 MYSQL 密码等
docker compose --profile infra --profile java --profile media up -d
```

首次启动后按 [`deploy/db/README.md`](media-platform-backend/deploy/db/README.md) 导入商品/订单表，
再执行 [`deploy/db/seed-media.sql`](media-platform-backend/deploy/db/seed-media.sql) 写入演示用户、视频与直播间。

### 前端容器（可选）

```bash
docker compose -f compose.yml -f compose.web.yml --profile web up -d --build
# http://localhost:8080
```

---

## 3. 已完成的功能

| 领域 | 能力 |
| --- | --- |
| 账号 | 注册（手机/邮箱验证码）、登录、**刷新单飞 + 跨标签锁**、登出；**资料编辑（昵称/头像/性别/简介）** |
| 点播 | 首页/搜索/播放页、HLS 播放、分片上传（并发 3 / 断点续传 / 退避重试）、封面上传、转码轮询、发布、**点播弹幕（按进度回放 / 发送 / WS）** |
| 互动 | 点赞、收藏（按收藏时间排序）、关注；**他人主页（作品 / 粉丝 / 关注）** |
| 直播 | 直播列表、直播间 HLS/FLV、20s 心跳、8s 在线轮询、弹幕 WS；**创建直播间 / 我的直播间 / 主播控制台（开始/结束、推流地址）** 全链路可用 |
| 商城 | 商品列表、SKU 选择、下单、模拟支付、订单列表与详情；**商品管理（建品/SKU/上下架/改价/补库存）** |
| 创作 | **视频管理**（我的投稿、编辑、下架、删除） |
| 管理 | **管理后台**：分类创建、敏感词词库、强制关播、按 userId 查用户、视频下架（需 ADMIN） |
| 体验 | 深浅色主题、骨架屏与空态、未登录 `returnTo` 回跳、`prefers-reduced-motion` |

### 明确未实现（当前版本）

- **评论 / 投币**：产品决策不做
- **真实支付 / 退款售后 / 通知 / 推荐 / 搜索增强 / 清晰度手动切换**
- **粉丝/关注分页滚动加载**（当前只取第一页 20 条）
- **用户封禁 / 审核队列**（管理后台只做查询与内容处置）

---

## 4. 文档索引

| 文档 | 内容 |
| --- | --- |
| [前端设计-后端前置问题与代码架构.md](前端设计-后端前置问题与代码架构.md) | 最初的可行性分析：后端缺口、前端架构 |
| [后端前置任务.md](后端前置任务.md) | Gate 1–3 任务清单、验收标准、排期与实时进度 |
| [开发问题记录.md](开发问题记录.md) | 代码 / 业务逻辑问题（联调 I、后端 B、前端 F） |
| [开发环境问题.md](开发环境问题.md) | 构建、脚本、编码、沙箱等非业务问题（E） |
| [web/docs/UI设计规范.md](web/docs/UI设计规范.md) | 设计 token、组件规则、页面截图与 P3 待办 |
| [契约约定.md](media-platform-backend/docs/契约约定.md) | 信封、时间、分页、空值、ID、枚举、鉴权、媒体地址 |
| [web/README.md](web/README.md) | 前端目录分层与本地开发说明 |

---

## 5. 真实联调验证（已完成）

2026-10-26 在本机起全栈（MySQL / Redis / RabbitMQ / MinIO / Nacos / SRS + 8 个服务）跑通：

| 链路 | 结果 |
| --- | --- |
| 注册 → 登录 → `/users/me` | ✅ 经网关，JWT 注入身份 |
| 公开资料 `/profiles`、关注/关系/粉丝 | ✅ 批量与单条均正常 |
| 点赞 / 取消 / 收藏 / 收藏列表 | ✅ 幂等、计数正确、收藏按时间倒序 |
| 分片上传 → 真实 FFmpeg 转码 360p/720p/1080p → 发布 → HLS 播放 | ✅ master.m3u8 200/206，ts 可取 |
| 商品创建 → SKU → 上架 → 下单 → mock-pay → 订单详情 | ✅ 状态流转正确 |
| 直播房间 / 心跳 / 在线人数 / 弹幕 REST + WebSocket | ✅ 中文弹幕、welcome 身份、广播均正常 |
| 浏览器 E2E（真实后端） | ✅ `pnpm e2e:real`：2 passed（含浏览器内上传发布、真实 HLS 播放） |
| 浏览器 E2E（mock） | ✅ `pnpm e2e`：4 passed |

联调中暴露并修复的 10 个问题（profiles 缺路由、Dubbo 实例名冲突、`/live` 路由与媒体代理冲突、
refresh 字段名不一致、ffmpeg 工作目录相对路径、在线人数缓存缺失等）全部记录在
[开发问题记录.md](开发问题记录.md)，可复现步骤见
[本地联调.md](media-platform-backend/docs/本地联调.md)。

---

## 6. 已知限制

1. **转码演示必须 `VIDEO_TRANSCODER_DRIVER=real`**：`mock` 只生成结构正确但没有分片的 m3u8，播放器会一直转圈。
2. **Windows + 受限终端构建**：若 `pnpm build` 报 `spawn EPERM` / 删临时文件被拒，把 `TEMP` 指向工作区目录再跑：
   `$env:TEMP="$PWD\..\.tmp-build"; pnpm build`（本机与 CI 通常不需要）。
3. **`mock-pay` 只在本地/演示开启**（`MOCK_PAY_ENABLED`），生产必须关闭。
4. **E2E 首次需装浏览器**：`pnpm exec playwright install chromium`。
5. **浏览器不能直接 RTMP 推流**：主播用 OBS，直播间页已提供「开播 + 复制推流地址 + 状态轮询」控制台。
6. **种子直播间没有真实推流**：`seed-media.sql` 的房间状态是 `LIVING`，但播放器会等流；
   在线人数来自心跳，不是真实观众。要验证完整直播链路需 OBS 或 `/live/dev/publish`。

---

## 7. 技术栈

- 前端：React 18 · TypeScript 5 · Vite 6 · TanStack Query 5 · Zustand 5 · Tailwind 3 · hls.js
- 后端：Spring Boot 3 / Spring Cloud Gateway / MyBatis（Java）· Gin / GORM / MinIO SDK（Go）
- 基础设施：MySQL 8 · Redis 7 · Nacos · RabbitMQ · MinIO · SRS 5
