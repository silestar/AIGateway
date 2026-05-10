# Changelog

## [0.2.0] - 2026-05-10

### 模型配置页面全面整合
- 模型配置弹窗（ModelSelectModal）功能全部整合进渠道详情「模型配置」Tab 页，去掉了弹窗壳
- 供应商+模型选择、筛选/搜索、自定义模型输入、已选标签、模型映射全部在 Tab 页内完成
- 删除 ModelSelectModal.vue 文件

### 跨渠道自定义模型名自动补全
- 新增后端 API：`GET /channels/custom-model-names`，返回所有渠道已配置的自定义模型名（display != actual，去重）
- 前端自动补全候选项合并三个来源：跨渠道自定义名 + 当前渠道已有映射 + 当前未保存映射

### 模型设置双栏多选模式
- 已配置模型采用双栏多选布局：左栏已启用 / 右栏已禁用
- 两栏均支持全选和批量移动
- 重置 / 保存按钮，仅在有变更时启用保存

### connection_decorator 系统级插件钩子
- 新增 `pkg/sdk/connection_decorator.go`：ConnectionDecorator 接口 + 全局注册表
- HookName 扩展：`HookConnectionDecorator`
- Plugin 模型扩展：`PluginType` 字段（sidecar/system）
- Manifest 扩展：`Type` 字段
- 代理引擎 NewEngine 中 DialTLSContext 注入 connection_decorator 调用
- 插件接口规范文档补充 system 类型 + connection_decorator 钩子说明

## [0.1.2] - 2026-05-10

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

### Bug 修复与体验优化

- 修复渠道分组创建后左侧列表统计数不刷新的问题：`handleAddChannels` 中 `selectCG()` 前新增 `await loadChannelGroups()`
- 修复密钥分组创建 500 错误：`autoMigrate` 缺少 `KeysGroupChannelGroup` 表注册
- 修复渠道账号创建时可重复添加相同密钥的问题：`Create()` 新增同渠道下密钥去重检测
- 修复渠道编辑页面优先级默认值矛盾：`NInputNumber` 的 `min` 从 0 改为 1，与后端"不允许填 0"逻辑对齐
- 新增渠道模型选择自定义模型输入功能：`ModelSelectModal` 顶部添加输入框 + 添加按钮，支持手动输入未抓取到的上游模型
- 修复 GitHub Models 等无 `/v1/models` 端点的 openai 兼容渠道测试连接 404：`TestConnection` 对 openai 类型改用 `/v1/chat/completions` 轻量请求测试连通性
- 渠道编辑页面新增字段提示文案：权重、RPM 限制、TPM 限制、每日请求上限均添加 `feedback` 说明（强调每个账号独立计数）
