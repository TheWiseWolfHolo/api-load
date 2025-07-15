# 默认目标
.DEFAULT_GOAL := help

# ==============================================================================
# 运行与开发
# ==============================================================================
.PHONY: run
run: ## 构建前端并运行服务器
	@echo "--- Building frontend... ---"
	cd web && npm install && npm run build
	@echo "--- Preparing backend... ---"
	@echo "--- Starting backend... ---"
	go run ./main.go

.PHONY: dev
dev: ## 以开发模式运行（带竞态检测）
	@echo "🔧 开发模式启动..."
	go run -race ./main.go

.PHONY: help
help: ## 显示此帮助信息
	@awk 'BEGIN {FS = ":.*?## "; printf "Usage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?## / { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
