# 公网部署：Nginx / TLS / 端口暴露

目标拓扑：**只有 443（HTTPS）与 1936（RTMP）对公网开放**，其余全部绑在 `127.0.0.1`。
浏览器只访问 `https://your.site`，页面与接口同源，因此不需要给网关配 CORS。

```
浏览器 ──443──> 宿主机 Nginx ──┬── /            → 前端 dist（或 web 容器 :8080）
                              ├── /api, /ws    → 127.0.0.1:8000  gateway
                              ├── /media       → 127.0.0.1:9000  MinIO（剥前缀）
                              └── /live        → 127.0.0.1:8088  SRS（剥前缀）
主播 OBS ──1936──> SRS RTMP
```

---

## 1. 两种前端部署方式

### 方式 A（推荐）：宿主机 Nginx 直接托管 dist

```bash
cd web && pnpm build          # 产物在 web/dist
sudo cp -r web/dist/* /usr/share/nginx/html/
sudo cp media-platform-backend/deploy/nginx/media-platform.conf /etc/nginx/conf.d/media-platform.conf
sudo nginx -t && sudo systemctl reload nginx
```

`media-platform.conf` 里 `root /usr/share/nginx/html;` 对应上面的目录。

### 方式 B：前端跑容器，宿主机 Nginx 只做 TLS 终结

```bash
docker compose --profile web up -d --build   # web 容器监听 127.0.0.1:8080
```

此时宿主机 Nginx 只需要一段「全量反代」：

```nginx
server {
    listen 443 ssl http2;
    server_name your.site;
    ssl_certificate     /etc/nginx/certs/fullchain.pem;
    ssl_certificate_key /etc/nginx/certs/privkey.pem;

    client_max_body_size 16m;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 3600s;
    }
}
```

> 方式 B 下 `/api`、`/media`、`/live` 由 **web 容器内的 nginx.conf** 按服务名转发，
> 不再需要宿主机上的分路径配置。

---

## 2. 证书

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d your.site
```

证书路径按实际替换 `media-platform.conf` 里的 `ssl_certificate*`；
`certbot renew` 由系统 timer 自动续期，续期后 `nginx -s reload`。

---

## 3. 防火墙 / 安全组

| 端口 | 来源 | 用途 |
| --- | --- | --- |
| 443 | 0.0.0.0/0 | HTTPS（唯一 Web 入口） |
| 80 | 0.0.0.0/0 | 仅用于 301 跳 443 + ACME 校验 |
| 1936 | 0.0.0.0/0 | RTMP 推流（OBS） |
| 8000–8004 / 9001–9003 / 3307 / 6380 / 5672 / 9000 / 8848 | **不开放** | 全部只绑 127.0.0.1 |

`compose.java.yml` / `compose.go.yml` 里的 `ports` 已经写成 `127.0.0.1:xxxx:xxxx`，
不要为了「方便调试」改成 `0.0.0.0`：Go 服务信任网关注入的 `X-User-Id`，直接暴露等于可被冒充。

---

## 4. 与媒体地址的联动（最容易错）

| 变量 | 生产值 | 说明 |
| --- | --- | --- |
| `VIDEO_MINIO_PUBLIC_ENDPOINT` | `https://your.site/media` | 视频/封面回给浏览器的前缀；Nginx 的 `location /media/` 会剥掉 `/media` |
| `LIVE_PUBLIC_BASE` | `https://your.site/live` | HLS/FLV 地址前缀；Nginx 的 `location /live/` 会 rewrite 掉 `/live` |
| `PUBLIC_SCHEME` / `PUBLIC_HOST` | `https` / `your.site` | 其余绝对 URL 使用 |

少写 `/media` → 播放器 404；用 `http://` 前缀 → HTTPS 页面出现混合内容被浏览器拦截。

### 路径拼接（容易看错，实测过）

后端返回的是「`PUBLIC_ENDPOINT` + `/` + bucket + `/` + objectKey」：

| 环境 | `VIDEO_MINIO_PUBLIC_ENDPOINT` | 实际 URL | Nginx 剥前缀后 |
| --- | --- | --- | --- |
| 本地直连 MinIO | `http://127.0.0.1:9000` | `/media/video/hls/9013/master.m3u8` | ——（不经过 Nginx） |
| 生产同源 | `https://your.site/media` | `/media/media/video/hls/9013/master.m3u8` | `/media/video/hls/9013/master.m3u8` ✅ |

所以生产 URL 里会看到**两个 `media`**：第一个是 Nginx 的路由前缀，第二个是 MinIO 的 bucket 名。
如果只有一段 `/media/...`，Nginx 会把 bucket 也剥掉，MinIO 返回 `AccessDenied`（本地 TLS 冒烟时踩过）。

直播同理：`LIVE_PUBLIC_BASE=https://your.site/live` + `srsApp=live` →
`https://your.site/live/live/<streamKey>.m3u8`，Nginx 剥掉第一个 `/live/` 后是 SRS 的 `/live/<streamKey>.m3u8`。

---

## 5. 上线前检查清单

- [ ] `.env.production` 权限 600，未提交仓库；`JWT_SECRET` 与 `PAYMENT_CALLBACK_SECRET` 为随机值
- [ ] `MOCK_PAY_ENABLED=false`、`AUTH_VERIFICATION_EXPOSE_CODE=false`
- [ ] `docker compose --env-file .env.production --profile java --profile infra --profile media up -d`
- [ ] `curl -I https://your.site/api/v1/videos` 返回 200
- [ ] 浏览器控制台无 CORS / 混合内容报错
- [ ] 上传一个视频，确认是 `https://your.site/media/...` 而不是 `http://IP:9000`
- [ ] 防火墙只放行 80/443/1936

---

## 6. 本地 TLS 冒烟（已通过，2026-10-26）

用自签证书 + `nginx:1.27-alpine` 在 `127.0.0.1:8443` 复刻了本文件的 443 配置（只把
`127.0.0.1` 换成 `host.docker.internal`、证书路径换成本地自签文件），验证结果：

| 请求 | 结果 |
| --- | --- |
| `GET /` | 200，返回前端 `index.html` |
| `GET /streams/9501` | 200，SPA fallback 命中 |
| `GET /api/v1/videos?page=1&pageSize=2` | 200，JSON 信封正常 |
| `GET /api/v1/profiles?uids=1001` | 200，返回演示用户 |
| `GET /media/media/video/hls/9013/master.m3u8` | 200，HLS master 可取 |
| `WSS /ws/danmaku?roomId=9501` | 握手成功，收到 `welcome`，uid 由网关从 token 注入 |

> 自签证书仅供冒烟；公网必须用 Let's Encrypt 或商业证书，浏览器才不会拦。
