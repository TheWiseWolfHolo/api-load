# GPT-Load

![Docker Build](https://github.com/tangbo/gpt-load/actions/workflows/docker-build.yml/badge.svg)
![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)

一个**极致高性能**的 OpenAI API 多密钥轮询透明代理服务器，使用 Go 语言开发！

## ✨ 特性

- 🔄 **多密钥轮询**: 自动轮换使用多个 API 密钥，支持负载均衡
- 🧠 **智能拉黑**: 区分永久性错误和临时性错误，智能密钥管理
- 📊 **实时监控**: 提供详细的统计信息、健康检查和黑名单管理
- 🔧 **灵活配置**: 支持 .env 文件配置，热重载配置
- 🌐 **CORS 支持**: 完整的跨域请求支持
- 📝 **结构化日志**: 彩色日志输出，包含响应时间和密钥信息
- 🔒 **可选认证**: 项目级 Bearer Token 认证
- ⚡ **极致性能**:
  - **零拷贝流式传输**: 最小化内存使用和延迟
  - **高并发处理**: 支持数万并发连接
  - **内存安全**: 自动垃圾回收，无内存泄漏
  - **原子操作**: 无锁并发，极低延迟
- 🛡️ **生产就绪**: 优雅关闭、错误恢复、内存管理

## 🚀 快速开始

### 方式一：直接运行

```bash
# 1. 确保已安装 Go 1.21+
go version

# 2. 下载依赖
go mod tidy

# 3. 配置密钥文件
cp ../keys.txt ./keys.txt

# 4. 配置环境变量（可选）
cp .env.example .env

# 5. 运行服务器
make run
# 或者
go run cmd/main.go
```

### 方式二：构建后运行

```bash
# 构建
make build

# 运行
./build/gpt-load
```

### 方式三：Docker 运行

#### 使用预构建镜像（推荐）

```bash
# 从GitHub Container Registry拉取镜像
docker pull ghcr.io/tangbo/gpt-load:latest

# 运行容器
docker run -d -p 3000:3000 \
  -e KEYS_FILE=/app/keys.txt \
  -v $(pwd)/keys.txt:/app/keys.txt \
  ghcr.io/tangbo/gpt-load:latest
```

#### 本地构建镜像

```bash
# 构建镜像
make docker-build

# 运行容器
make docker-run

# 或使用 docker-compose
docker-compose up -d
```

## ⚙️ 配置管理

### 环境变量配置

```bash
cp .env.example .env
```

### 主要配置项

| 配置项     | 环境变量              | 默认值                 | 说明                  |
| ---------- | --------------------- | ---------------------- | --------------------- |
| 服务器端口 | `PORT`                | 3000                   | 服务器监听端口        |
| 服务器主机 | `HOST`                | 0.0.0.0                | 服务器绑定地址        |
| 密钥文件   | `KEYS_FILE`           | keys.txt               | API 密钥文件路径      |
| 起始索引   | `START_INDEX`         | 0                      | 从哪个密钥开始轮询    |
| 拉黑阈值   | `BLACKLIST_THRESHOLD` | 1                      | 错误多少次后拉黑      |
| 上游地址   | `OPENAI_BASE_URL`     | https://api.openai.com | OpenAI API 地址       |
| 请求超时   | `REQUEST_TIMEOUT`     | 30000                  | 请求超时时间（毫秒）  |
| 认证密钥   | `AUTH_KEY`            | 无                     | 项目认证密钥（可选）  |
| CORS       | `ENABLE_CORS`         | true                   | 是否启用 CORS         |
| 连接池     | `MAX_SOCKETS`         | 50                     | HTTP 连接池最大连接数 |

## 📊 监控端点

| 端点          | 方法 | 说明               |
| ------------- | ---- | ------------------ |
| `/health`     | GET  | 健康检查和基本状态 |
| `/stats`      | GET  | 详细统计信息       |
| `/blacklist`  | GET  | 黑名单详情         |
| `/reset-keys` | GET  | 重置所有密钥状态   |

## 🚀 CI/CD 自动部署

项目已配置 GitHub Actions 自动构建和部署：

- **自动触发**：每次 push 到 main 分支时自动构建
- **多平台支持**：同时构建 Linux AMD64 和 ARM64 镜像
- **GitHub 集成**：自动推送到 GitHub Container Registry
- **缓存优化**：使用 GitHub Actions 缓存加速构建

### 预构建镜像

```bash
# 拉取最新镜像
docker pull ghcr.io/tangbo/gpt-load:latest
```

详细配置说明请参考：[GitHub Actions 配置指南](.docs/github-actions-setup.md)

## 🔧 开发指南

### 可用命令

```bash
# 构建相关
make build      # 构建二进制文件
make build-all  # 构建所有平台版本
make clean      # 清理构建文件

# 运行相关
make run        # 运行服务器
make dev        # 开发模式运行（启用竞态检测）

# 测试相关
make test       # 运行测试
make coverage   # 生成测试覆盖率报告
make bench      # 运行基准测试

# 代码质量
make lint       # 代码检查
make fmt        # 格式化代码
make tidy       # 整理依赖

# 管理相关
make health     # 健康检查
make stats      # 查看统计信息
make reset-keys # 重置密钥状态
make blacklist  # 查看黑名单

# 查看所有命令
make help
```

### 项目结构

```
/
├── cmd/
│   └── main.go              # 主入口文件
├── internal/
│   ├── config/
│   │   └── config.go        # 配置管理
│   ├── keymanager/
│   │   └── keymanager.go    # 密钥管理器
│   └── proxy/
│       └── proxy.go         # 代理服务器核心
├── build/                   # 构建输出目录
├── .env.example            # 配置文件模板
├── Dockerfile              # Docker 构建文件
├── docker-compose.yml      # Docker Compose 配置
├── Makefile               # 构建脚本
├── go.mod                 # Go 模块文件
└── README.md              # 项目文档
```

## 🏗️ 架构设计

### 高性能设计

1. **并发模型**: 使用 Go 的 goroutine 实现高并发处理
2. **内存管理**: 零拷贝流式传输，最小化内存分配
3. **连接复用**: HTTP/2 支持，连接池优化
4. **原子操作**: 无锁并发，避免竞态条件
5. **预编译正则**: 启动时预编译，避免运行时开销

### 安全设计

1. **内存安全**: Go 的内存安全保证，避免缓冲区溢出
2. **并发安全**: sync.Map 和原子操作保证并发安全
3. **错误处理**: 完整的错误处理和恢复机制
4. **资源清理**: 自动资源清理，防止泄漏
