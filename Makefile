# 变量定义
BINARY_NAME=gpt-load
MAIN_PATH=./cmd/gpt-load
BUILD_DIR=./build
VERSION=2.0.0
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -s -w"

# 从 .env 文件加载环境变量，如果不存在则使用默认值
HOST ?= $(shell sed -n 's/^HOST=//p' .env 2>/dev/null || echo "localhost")
PORT ?= $(shell sed -n 's/^PORT=//p' .env 2>/dev/null || echo "3000")
API_BASE_URL=http://$(HOST):$(PORT)

# 默认目标
.DEFAULT_GOAL := help

.PHONY: all
all: clean build ## 清理并构建项目

# ==============================================================================
# 构建相关命令
# ==============================================================================
.PHONY: build
build: ## 构建二进制文件
	@echo "🔨 构建 $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "✅ 构建完成: $(BUILD_DIR)/$(BINARY_NAME)"

.PHONY: build-all
build-all: clean ## 为所有支持的平台构建二进制文件
	@echo "🔨 构建所有平台版本..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "✅ 所有平台构建完成"

# ==============================================================================
# 运行与开发
# ==============================================================================
.PHONY: run
run: ## 构建前端并运行服务器
	@echo "--- Building frontend... ---"
	@rm -rf cmd/gpt-load/dist
	cd web && pnpm install && pnpm run build
	@echo "--- Preparing backend... ---"
	@echo "--- Starting backend... ---"
	go run $(MAIN_PATH)/main.go

.PHONY: dev
dev: ## 以开发模式运行（带竞态检测）
	@echo "🔧 开发模式启动..."
	go run -race $(MAIN_PATH)/main.go

# ==============================================================================
# 测试与代码质量
# ==============================================================================
.PHONY: test
test: ## 运行所有测试
	@echo "🧪 运行测试..."
	go test -v -race -coverprofile=coverage.out ./...

.PHONY: coverage
coverage: test ## 生成并查看测试覆盖率报告
	@echo "📊 生成测试覆盖率报告..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ 覆盖率报告生成: coverage.html"

.PHONY: bench
bench: ## 运行基准测试
	@echo "⚡ 运行基准测试..."
	go test -bench=. -benchmem ./...

.PHONY: lint
lint: ## 使用 golangci-lint 检查代码
	@echo "🔍 代码检查..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️ golangci-lint 未安装，跳过代码检查"; \
		echo "安装命令: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

.PHONY: fmt
fmt: ## 格式化 Go 代码
	@echo "🎨 格式化代码..."
	go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "💡 建议安装 goimports: go install golang.org/x/tools/cmd/goimports@latest"; \
	fi

.PHONY: tidy
tidy: ## 整理和验证模块依赖
	@echo "📦 整理依赖..."
	go mod tidy
	go mod verify

.PHONY: deps
deps: ## 下载模块依赖
	@echo "📥 安装依赖..."
	go mod download

# ==============================================================================
# 清理与安装
# ==============================================================================
.PHONY: clean
clean: ## 清理所有构建产物
	@echo "🧹 清理构建文件..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

.PHONY: install
install: build ## 构建并安装二进制文件到 /usr/local/bin
	@echo "📦 安装到系统..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "✅ 安装完成: /usr/local/bin/$(BINARY_NAME)"

.PHONY: uninstall
uninstall: ## 从 /usr/local/bin 卸载二进制文件
	@echo "🗑️ 从系统卸载..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "✅ 卸载完成"

# ==============================================================================
# Docker 相关命令
# ==============================================================================
.PHONY: docker-build
docker-build: ## 构建 Docker 镜像
	@echo "🐳 构建 Docker 镜像..."
	docker build -t gpt-load:$(VERSION) .
	docker tag gpt-load:$(VERSION) gpt-load:latest
	@echo "✅ Docker 镜像构建完成"

.PHONY: docker-run
docker-run: ## 使用预构建镜像运行 Docker 容器
	@echo "🐳 运行 Docker 容器（预构建镜像）..."
	docker run -d \
		--name gpt-load \
		-p 3000:3000 \
		-v $(PWD)/keys.txt:/app/keys.txt:ro \
		-v $(PWD)/.env:/app/.env:ro \
		--restart unless-stopped \
		ghcr.io/tbphp/gpt-load:latest

.PHONY: docker-run-local
docker-run-local: ## 使用本地构建的镜像运行 Docker 容器
	@echo "🐳 运行 Docker 容器（本地构建）..."
	docker run -d \
		--name gpt-load-local \
		-p 3000:3000 \
		-v $(PWD)/keys.txt:/app/keys.txt:ro \
		-v $(PWD)/.env:/app/.env:ro \
		--restart unless-stopped \
		gpt-load:latest

.PHONY: compose-up
compose-up: ## 使用 Docker Compose 启动（预构建镜像）
	@echo "🐳 使用 Docker Compose 启动（预构建镜像）..."
	docker-compose up -d

.PHONY: compose-up-dev
compose-up-dev: ## 使用 Docker Compose 启动（本地构建）
	@echo "🐳 使用 Docker Compose 启动（本地构建）..."
	docker-compose -f docker-compose.dev.yml up -d

.PHONY: compose-down
compose-down: ## 停止所有 Docker Compose 服务
	@echo "🐳 停止 Docker Compose..."
	docker-compose down
	docker-compose -f docker-compose.dev.yml down 2>/dev/null || true

# ==============================================================================
# 服务管理与工具
# ==============================================================================
.PHONY: validate-keys
validate-keys: ## 验证 API 密钥的有效性
	@echo "🐍 使用 Python 版本验证密钥..."
	python3 scripts/validate-keys.py -c 100 -t 15

.PHONY: health
health: ## 检查服务的健康状况
	@echo "💚 健康检查..."
	@curl -s $(API_BASE_URL)/health | jq . || echo "请安装 jq 或检查服务是否运行"

.PHONY: stats
stats: ## 查看服务的统计信息
	@echo "📊 查看统计信息..."
	@curl -s $(API_BASE_URL)/stats | jq . || echo "请安装 jq 或检查服务是否运行"

.PHONY: reset-keys
reset-keys: ## 重置所有密钥的状态
	@echo "🔄 重置密钥状态..."
	@curl -s $(API_BASE_URL)/reset-keys | jq . || echo "请安装 jq 或检查服务是否运行"

.PHONY: blacklist
blacklist: ## 查看当前黑名单中的密钥
	@echo "🚫 查看黑名单..."
	@curl -s $(API_BASE_URL)/blacklist | jq . || echo "请安装 jq 或检查服务是否运行"

.PHONY: help
help: ## 显示此帮助信息
	@awk 'BEGIN {FS = ":.*?## "; printf "Usage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?## / { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
