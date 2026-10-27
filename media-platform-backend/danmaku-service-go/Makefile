# danmaku-service-go 常用命令
#
# 数据层与 Java 体系共用：默认连接 127.0.0.1:3306/auth_service 与 127.0.0.1:6379 db0。
# 本地独立调试时 live-service 可能没在跑，因此 danmaku.require_living 默认为 false
# （只有显式收到 live.stopped 才拒绝发送）；线上建议置 true。

APP      := danmaku-service
PORT     := 9003
BIN      := bin/server
CONFIG   := configs/config.yaml

.PHONY: help run build test test-race vet fmt tidy clean docker health ws-check

help: ## 显示可用命令
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

run: ## 本地启动（前台运行）
	go run ./cmd/server -config $(CONFIG)

build: ## 编译到 bin/server
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd/server

test: ## 运行单元测试（含 hub 并发、敏感词、限流）
	go test ./...

test-race: ## 带竞态检测的测试（hub / filter 的并发逻辑必跑）
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

health: ## 探针自检（需服务已启动；关注 filter 一项的 words 数量）
	curl.exe -s http://localhost:$(PORT)/healthz
	curl.exe -s http://localhost:$(PORT)/readyz

ws-check: ## 发一条弹幕并回读历史（验证发送 → 热数据 → 历史链路）
	curl.exe -s -X POST http://localhost:$(PORT)/api/v1/danmaku/send -H "X-User-Id: u-2001" -H "Content-Type: application/json" -d "{\"roomId\":\"5001\",\"content\":\"make 自检弹幕\",\"clientMsgId\":\"make-0001\"}"
	curl.exe -s "http://localhost:$(PORT)/api/v1/danmaku/rooms/5001/history?page=1&pageSize=5"
