# 多阶段构建：Node 构建静态产物 → Nginx 同源反代
FROM node:22-alpine AS build
WORKDIR /app

# 先只拷依赖清单，命中缓存时跳过重复安装
COPY package.json pnpm-lock.yaml .npmrc ./
RUN corepack enable && pnpm install --frozen-lockfile

COPY . .
RUN pnpm build

FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
