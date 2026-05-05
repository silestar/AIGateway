.PHONY: build run test lint clean setup

# Go 环境
GOROOT := /home/workspace/.env/go
GOPATH := /home/workspace/.env/gopath
export PATH := $(GOROOT)/bin:$(GOPATH)/bin:$(PATH)

# 项目配置
BINARY := bin/agw
PORT := 7860

## setup: 安装 Go 环境（首次运行）
setup:
	@if [ ! -f $(GOROOT)/bin/go ]; then \
		echo "Installing Go 1.22.10 (arm64)..."; \
		mkdir -p /home/workspace/.env/gopath; \
		curl -sL https://go.dev/dl/go1.22.10.linux-arm64.tar.gz -o /tmp/go.tar.gz; \
		tar -C /home/workspace/.env -xzf /tmp/go.tar.gz; \
		rm /tmp/go.tar.gz; \
		echo "Go installed: $(GOROOT)/bin/go"; \
	else \
		echo "Go already installed: $(GOROOT)/bin/go"; \
	fi

## build: 编译项目
build:
	go build -o $(BINARY) ./cmd/agw

## run: 运行项目
run: build
	./$(BINARY)

## dev: 开发模式运行（热重载需 air）
dev:
	go run ./cmd/agw

## test: 运行测试
test:
	go test ./... -v

## lint: 代码检查
lint:
	go vet ./...

## tidy: 整理依赖
tidy:
	go mod tidy

## clean: 清理构建产物
clean:
	rm -rf bin/ data/ logs/

## frontend: 安装前端依赖
frontend:
	cd web && npm install

## frontend-dev: 启动前端开发服务器
frontend-dev:
	cd web && npm run dev

## frontend-build: 构建前端
frontend-build:
	cd web && npm run build

## help: 显示帮助
help:
	@echo "AIGateway Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
