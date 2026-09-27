# video-service-go 常用命令
#
# 数据层与 Java 体系共用：默认就连接 127.0.0.1:3306/auth_service 与 127.0.0.1:6379 db0，
# 因此本地开发**不需要**额外起 MySQL/Redis（没有 Java 那套时用
# `docker compose --profile standalone up -d` 并在 .env 里改 DSN / GO_REDIS_ADDR）。

APP      := video-service
PORT     := 9001
BIN      := bin/server
CONFIG   := configs/config.yaml

.PHONY: help run build test test-race vet fmt tidy clean docker health

help: ## 显示可用命令
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

run: ## 本地启动（前台运行）
	go run ./cmd/server -config $(CONFIG)

build: ## 编译到 bin/server
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd/server

test: ## 运行单元测试
	go test ./...

test-race: ## 带竞态检测的测试（涉及并发/缓存的改动必须跑）
	go test -race ./...

vet: ## 静态检查
	go vet ./...

fmt: ## 格式化
	gofmt -w .

tidy: ## 整理依赖
	go mod tidy

clean: ## 清理编译产物
	rm -rf bin

docker: ## 构建镜像
	docker build -t $(APP):local .

health: ## 探针自检（需服务已启动）
	curl.exe -s http://localhost:$(PORT)/healthz
	curl.exe -s http://localhost:$(PORT)/readyz
