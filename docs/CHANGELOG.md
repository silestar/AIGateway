# Changelog

## [0.1.0] - 2026-05-05

### 阶段一：基础设施层
- 项目骨架与目录结构
- 配置加载（Viper + .env）
- 存储抽象层（GORM + SQLite 默认）
- 全局加密服务（AES-256-GCM）
- 系统日志（zap，按日归档）
- Gin 路由骨架 + 基础中间件
- Vue 前端骨架

### 阶段二：代理引擎与渠道适配
- HTTP 代理引擎（连接池、超时、透明转发）
- 流式传输支持（SSE 透传）
- OpenAI 适配器（Chat Completions，零转换透传）
- 最小请求转发链路

### 阶段三：账号池核心逻辑
- 账号 CRUD + 优先级选择
- 粘性绑定
- 故障降级（连续失败计数 → disabled）
- 自动探测恢复（含冷却保护）
- 解密缓存
- 18 个单元测试全部通过

### 阶段四：路由与密钥模块
- 密钥管理 + API Key 认证 + RPM/TPM 配额
- 渠道分组 + 密钥分组管理
- 模型存在性过滤
- 分层确定性路由引擎
- 11 个分组路由单元测试

### 阶段五：管理 API 与 WebUI
- 8 个 RESTful 管理 API Handler
- Vue 前端 8 个页面（Dashboard/密钥/渠道/分组/统计/日志/插件/设置）
- 模型自动发现（FetchModels + SaveModels）
- 中英文国际化（vue-i18n）
- 前后端联调通过

### 阶段六：统计日志与可观测性
- 异步日志写入器（channel 缓冲 10000 + 批量 INSERT + 优雅关闭）
- 内存实时计数器（替代 Redis，单实例适用）
- 日聚合调度器（5 分钟一次，系统/密钥/渠道三级统计）
- StatsHandler + LogHandler 真实实现
- handleChatCompletions 集成日志记录
- 前端 Dashboard/Stats/Logs 对接真实 API
- 4 个单元测试通过

### 阶段七：扩展与完备
- Anthropic 适配器（Claude Messages API 协议转换 + 流式）
- Gemini 适配器（Google generateContent 协议转换 + 流式）
- 插件系统核心（Sidecar 进程管理 + 钩子调度 + Go SDK）
- Docker 多阶段构建 + docker-compose
- README + CHANGELOG

### 阶段八：插件优化与模型管理
- 插件注册中心：`marketplace_url/token` → `plugin_registry_url/use_registry_auth`，支持远程列表 + 一键安装
- 渠道类型插件：插件启动后通过 `/.well-known/channel-type` 自动发现并注册新渠道类型
- 前端渠道类型下拉框从 API 动态获取（不再硬编码），选插件类型时自动填充 base_url
- 模型管理模块：`model_catalog` 表 + 全量同步逻辑（SaveModels/Delete/UpdateStatus 触发）
- `/v1/models` 端点实现（OpenAI 兼容格式，返回 `visible=true` 的模型）
- 管理端 API：`GET /api/models/catalog`、`PUT /api/models/catalog/:id/visibility`
- 前端模型管理页面：左右两列（已选模型 / 自定义映射模型）+ 可见性开关
- 密钥分组配额语义修正："共享"→"各自的"（每密钥独立限额）

### 阶段九：仪表盘升级
- 后端 Dashboard API 扩展：`GET /api/stats/dashboard?days=7|30` 新增 `hourly_trend`、`top_models`、`top_channels`、`recent_errors` 字段
- Manager 新增 4 个聚合查询方法：`GetHourlyTrend`、`GetTopModels`、`GetTopChannels`、`GetRecentErrors`
- 前端仪表盘完全重写：5 列统计卡片（成功率/延迟颜色规则）、ECharts 折线趋势图（7天/30天切换）、环形饼图（模型分布）、横向柱状图（渠道负载）、异常请求表格（点击跳转日志）
- 自动刷新：30 秒轮询
- 颜色规则：成功率 >95% 绿/>80% 黄/<80% 红；延迟 <2s 绿/<5s 黄/>5s 红
- 中英文 i18n 扩展
