# live-service-go 常用命令
#
# 数据层与 Java 体系共用：默认连接 127.0.0.1:3306/auth_service 与 127.0.0.1:6379 db0。
# 本地没有 SRS 时用 mock 媒体驱动（默认），通过 dev 接口模拟推流回调：
#   curl.exe -X POST http://localhost:9002/api/v1/live/dev/publish/<streamKey>?session=<sessionId>

APP      := live-service
PORT     := 9002
BIN      := bin/server
CONFIG   := configs/config.yaml

.PHONY: help run build test test-race vet fmt tidy clean docker health media-up

help: ## 显示可用命令
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

run: ## 本地启动（mock 媒体驱动，无需 SRS）
	LIVE_MEDIA_DRIVER=mock go run ./cmd/server -config $(CONFIG)

build: ## 编译到 bin/server
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd/server

test: ## 运行单元测试
	go test ./...

test-race: ## 带竞态检测的测试
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

media-up: ## 启动 SRS（需要 docker，起在本机 1936/8088/1986）
	docker compose --profile media up -d srs
