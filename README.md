# AIGateway (AGW)

🚀 高性能多租户 AI API 聚合代理 — 统一 OpenAI 协议，智能路由，账号池管理

## ✨ 核心特性

- **统一协议**：入站出站均采用 OpenAI Chat Completions 格式，下游零适配
- **多适配器**：OpenAI（透传）、Anthropic（Claude）、Gemini 三种协议适配
- **智能路由**：分层确定性路由 — 消费者分组 → 渠道分组 → 渠道权重 → 账号优先级
- **账号池**：优先级选择、粘性绑定、故障降级、自动探测恢复
- **异步日志**：channel 缓冲 + 批量 INSERT，请求主链路零阻塞
- **实时统计**：内存原子计数器 + 日聚合调度，仪表盘实时展示
- **插件系统**：Sidecar 进程隔离，Go SDK 开箱即用，热插拔
- **模型发现**：一键 FetchModels 自动拉取上游模型列表，支持别名映射

## 🏗️ 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.22 + Gin + GORM + SQLite/MySQL/PostgreSQL |
| 前端 | Vue 3 + TypeScript + Naive UI + vue-i18n |
| 加密 | AES-256-GCM（密钥加密存储） |
| 日志 | zap（按日归档） |
| 部署 | Docker 多阶段构建 + docker-compose |

## 🚀 快速开始

### Docker 部署（推荐）

```bash
git clone https://github.com/silestar/AIGateway.git
cd aigateway
docker compose up -d
```

访问 `http://localhost:7860` 即可使用管理面板。

### 手动构建

```bash
# 后端
go build -buildvcs=false -o agw ./cmd/agw/
./agw

# 前端
cd web
npm install
npm run build
```

## 📖 使用方式

### 1. 创建渠道

在管理面板中创建渠道，填写上游 Base URL 和类型（openai/anthropic/gemini）。

### 2. 添加账号

在渠道详情页添加 API Key，系统自动加密存储。

### 3. 配置分组

创建消费者分组和渠道分组，设置权重，绑定关联关系。

### 4. 创建消费者

生成消费者 API Key，即可像使用 OpenAI API 一样调用：

```bash
curl http://localhost:7860/v1/chat/completions \
  -H "Authorization: Bearer your-consumer-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## 🏛️ 项目结构

```
.
├── cmd/agw/              # 入口
├── internal/
│   ├── account/          # 账号池管理
│   ├── api/              # RESTful API handlers
│   ├── channel/          # 渠道管理
│   ├── config/           # 配置加载
│   ├── consumer/         # 消费者管理 + 配额
│   ├── crypto/           # AES-256-GCM 加密
│   ├── group/            # 分组路由引擎
│   ├── i18n/             # 国际化
│   ├── log/              # zap 日志
│   ├── plugin/           # 插件管理器
│   ├── proxy/            # HTTP 代理引擎
│   ├── stats/            # 统计聚合
│   └── storage/          # 存储层抽象
├── pkg/
│   ├── adapter/          # 渠道适配器
│   │   ├── openai/       # OpenAI 透传
│   │   ├── anthropic/    # Claude Messages
│   │   ├── gemini/       # Google Gemini
│   │   └── registry/     # 适配器注册表
│   ├── middleware/        # Gin 中间件
│   └── plugin/sdk/       # Go 插件 SDK
├── web/                  # Vue 3 前端
├── Dockerfile
└── docker-compose.yml
```

## 🔒 安全

- 所有 API Key 使用 AES-256-GCM 加密存储
- SECRET_KEY 首次启动自动生成，持久化到 `.env`
- 密钥展示使用前缀脱敏（如 `sk-abc****`）
- 插件 Sidecar 绑定 127.0.0.1，Token 认证

## 📄 License

MIT
