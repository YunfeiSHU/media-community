# web · 前端

B 站风格音视频社区前端（React 18 + TypeScript + Vite + TanStack Query）。

## 快速开始

```bash
pnpm install
pnpm dev        # http://localhost:5173
pnpm build      # 生产构建（产物在 dist/）
pnpm e2e        # Playwright（首次需 pnpm exec playwright install chromium）
```

没有后端时默认走内置 mock（`.env.development` 里 `VITE_USE_MOCK=1`）。
后端起来后把该行删掉或设为 `0`，即自动切到真实接口。

## 目录

```
src/
├─ app/            # providers、router、layouts
├─ pages/          # 路由页面（home / search / video / not-found）
├─ widgets/        # 有业务语义的组合（SiteHeader / VideoCard / LiveCard / VideoPlayer）
├─ entities/       # 领域模型 + api + queries + adapters
├─ shared/         # api（http/errors/session/mock）、lib（datetime/money/status）、styles
```

依赖方向：`app → pages → widgets → entities → shared`，不允许反向 import。

## 与后端契约的约定

- 所有接口走 `/api/v1`，同源相对路径，不写死域名（生产由 Nginx 反代）。
- 响应信封 `{code,message,data}`，`code !== 0` 抛 `ApiError`。
- 时间用 `parseServerDate` 兼容三种后端格式；金额统一按「分」。
- 分页在 `entities/*/adapters.ts` 归一为 `{items,total,page,pageSize}`。

## 已知限制

- Windows 受限终端里若 `pnpm build` 报 `spawn EPERM` 或删除临时文件被拒，
  把 `TEMP/TMP` 指向工作区内目录即可（见《开发问题记录》E-003）；普通本机与 CI 不需要。
- 低延迟直播用 FLV（动态加载 `flv.js`），默认仍是 HLS。
- 评论、投币按产品决策**不做**，前端不设入口。
