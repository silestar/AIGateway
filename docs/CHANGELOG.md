# Changelog

## [0.2.0] - 2026-05-12

### 🏠 全新品牌首页
- 访问 `/` 不再跳转登录页，展示高端品牌首页
- 包含 Hero 区（品牌名、描述、CTA 按钮）、核心特性卡片（6 项）、技术栈展示
- 导航栏：首页 / 控制台 / 文档 / 关于
- 新增 Home.vue、Docs.vue（开发中占位）、About.vue
- 中英文 i18n 各新增 30+ key

### 🔐 登录升级：Token → 账户+密码
- 废弃旧的 `AGW_SERVER_API_TOKEN` 单 token 认证
- 改用 `AGW_ADMIN_USER` + `AGW_ADMIN_PASS` 账户密码登录
- `ServerConfig.APIToken` → `AdminUser` + `AdminPass`
- 登录页改为用户名+密码两个输入框
- 兼容提示：检测到旧 `AGW_SERVER_API_TOKEN` 时打印醒目升级警告

### 💾 Session 持久化
- 新增 `Session` GORM model（`sessions` 表），容器重启登录态不丢失
- 三层降级存储：Redis → SQLite → 内存 map
- 新增 `RedisSessionStore`（需配置 `redis.enabled=true`），TTL 自动过期
- 新增 `SQLiteSessionStore`，通过 `SessionStore` 接口实现
- 新增 `NewSessionStore()` 工厂函数，自动探测可用存储
- 每小时自动清理过期 session（SQLite/内存模式）

### 🔄 统一迁移入口
- 新增 `internal/config/migration.go` — 集中管理版本升级检测
- 启动流程：检查 `data/.agw_version` → 版本匹配跳过 / 不匹配执行迁移
- 状态标记：OK 0.2.0 / MIGRATING / FAILED，迁移中断后自动恢复
- 数据库备份机制：迁移前备份到 `data/backups/`，失败时自动恢复
- 旧备份自动清理：超过 30 天的备份文件自动删除
- 环境变量自动迁移：检测到旧 `AGW_SERVER_API_TOKEN` → 自动写入 `AGW_ADMIN_USER/PASS` 并删除旧行

### ⚠️ Breaking Changes
- `AGW_SERVER_API_TOKEN` 已废弃，请改用 `AGW_ADMIN_USER` + `AGW_ADMIN_PASS`


## [0.1.5] - 2026-05-12

### 失败关键词 UI 优化
- **Tag 只读展示 + 独立输入框**：将失败关键词输入从 `<n-dynamic-tags>` 改为 tag 只读展示（可关闭删除）+ 下方独立输入框 + 回车/按钮添加，解决 tag 内编辑文本框过短的问题
- **去重检测**：`addKeyword()` 添加前检查重复，避免同一条短句重复添加
- **术语修正**：i18n 中"失败关键词"改为"失败关键词/短句"，提示语同步更新为中英文
- **新增 i18n key**：`keywordsEmpty` / `keywordsPlaceholder` / `keywordsAdd` 中英文各 3 个

### 插件权限管理系统

- **权限声明**：插件在 `manifest.json` 中新增 `permissions` 字段，声明所需权限及是否必需（`required`）
- **11 个权限项**：`account_id` / `channel_id` / `keys_id` / `model_name` / `request_headers` / `request_body_summary` / `response_status` / `response_body_summary` / `server_info` / `channel_info` / `channel_config`
- **TriggerHook 数据过滤（P0 安全修复）**：`filterHookRequest` 根据授权结果过滤 `HookRequest` 字段，未授权字段置零/置空，无权限声明的插件照原样传递（向后兼容）
- **CONNECT 协议权限头部**：`dialViaDecorator` 根据授权结果携带 `X-AGW-Account-ID` / `X-AGW-Channel-ID` 等头部
- **管理员授权 UI**：插件卡片新增「权限」按钮 → 权限管理弹窗（Switch 开关 + 状态标签 + 授予/撤销/全部授予/全部撤销）
- **高敏感权限二次确认**：`request_headers` 和 `channel_config` 授予时弹窗警告
- **启动时权限检查**：`required: true` 的权限被拒绝时拒绝启动插件
- **自动授权模式**：`auto_grant_permissions: true` 时安装即授予所有权限（高敏感仍需二次确认）
- **权限缓存**：`permissionCache` + `sync.RWMutex`，管理 API 更新时实时刷新
- **插件升级同步**：`SyncPermissions` 处理新增/更新/删除的权限声明
- **卸载保留审计**：权限记录标记 `uninstalled` 但不删除
- **审计日志**：`plugin_permission_granted` / `denied` / `auto_granted` / `grant_all` / `deny_all` 结构化日志
- **API**：`GET /:id/permissions` / `PUT /:id/permissions/:permName/grant` / `deny` / `POST grant-all` / `deny-all`
- **前端 i18n**：中英文各新增 16 个 key

## [0.1.4] - 2026-05-12

### 渠道监控与自动处置系统

- **配置自动补全**：`EnsureConfigCompleteness` 启动时自动检测客户 config.yaml 缺失字段并补全，旧字段 `global_health_check_interval` 自动迁移到 `channel_health_check_interval`
- **401/403 立即禁用**：`ReportResult` 中匹配 `channel_disable_status_codes`（默认 401/403）时跳过连续失败计数逻辑，直接禁用账号
- **关键词匹配禁用**：`CheckDisableKeywords` 在 engine.go 错误路径检查上游响应体，匹配到关键词（不区分大小写）时自动禁用账号。默认覆盖 11 个常见封号/欠费/认证失败关键词
- **响应超时禁用**：`channel_disable_latency_threshold` 非流式请求响应时间超阈值时累积失败计数（仅非流式，流式含推理时间易误伤）
- **主动探测增强**：`healthCheckChannel` 两阶段逻辑——第一阶段恢复 disabled/cooling 账号，第二阶段对 active 账号主动探测
- **请求体重试修复**：`ForwardStream` / `Forward` 重试时 `c.Request.Body` 已被首次请求消耗导致空 body，改为缓存 `reqBodyBytes` 并在每次重试前重置
- **Accept-Encoding 过滤**：请求发给上游前移除 `Accept-Encoding: gzip`，防止上游返回 gzip 压缩的 502 导致 JSON 解码失败
- **流式读取超时**：`stream_read_timeout` 配置项 + `SetReadDeadline` 防止流式请求长时间无数据卡死
- **前端监控配置页面**：新增 `SystemMonitor.vue`（`/settings/monitor`），5 个分组卡片——定期渠道测试 / 响应时间限制 / 自动禁用状态码 / 自动重试状态码 / 失败关键词
- **前端菜单与路由**：系统子菜单新增「监控」入口，i18n 中英文各新增 18 个 key
- **Settings 页面修正**：`global_health_check_interval` → `channel_health_check_interval`

### AccountManagerConfig 新增字段

- `channel_health_check_interval`（默认 43200 秒 = 12h，替代废弃的 `global_health_check_interval`）
- `channel_disable_latency_threshold`（默认 0 = 不启用，单位秒）
- `channel_disable_on_failure`（默认 true）
- `channel_enable_on_success`（默认 true）
- `channel_disable_status_codes`（默认 [401, 403]）
- `channel_retry_status_codes`（默认 [502, 503, 504]，仅展示暂未实现重试逻辑）
- `channel_disable_keywords`（默认 11 个关键词，不区分大小写匹配）

## [0.1.3] - 2026-05-11

### 流式 Token 统计修复
- 修复流式请求 token 统计为 0 的问题：`ForwardStream` 自动注入 `stream_options: {"include_usage": true}`，让上游在流式最后一个 chunk 返回 usage 数据
- 仅在请求体未包含 `stream_options` 时注入，已有则不覆盖

### 缓存命中 Token 提取与展示
- `TokenUsage` 结构体新增 `CachedTokens` 字段
- 新增 `extractCachedTokens()` 函数，支持 OpenAI 格式（`prompt_tokens_details.cached_tokens`）和 Anthropic 格式（`cache_read_input_tokens`）
- 非流式和流式两条提取路径均已接入缓存提取逻辑
- `buildRequestLog` 写入 `CacheTokens` 到数据库，前端日志表格和详情面板均展示缓存命中数值，使用逗号分隔格式

### Bug 修复
- 修复模型设置页面保存失败：`catalog_service` 中 `BatchSetUpstreamVisible` / `BatchSetDisplayVisible` 使用 `Model(&gorm.Model{})` 导致 GORM 自动注入 `updated_at` 和 `deleted_at`，但 `channel_models` 表无此两列。改为直接 `.Table("channel_models")` 操作

## [0.1.2] - 2026-05-10

### 插件系统：Sidecar TCP 代理模式（重大架构升级）
- 抛弃 system 类型，全面改为 sidecar 模式，满足「插件非空壳 + AGW 零依赖」两条铁律
- 移除 `pkg/sdk.ConnectionDecorator` 接口/注册表 + `cmd/agw/main.go` blank import
- 代理引擎 `DialTLSContext` 改为查询数据库获取启用的 connection_decorator 插件地址
- 新增 `dialViaDecorator()` + CONNECT 协议转发，插件不可用时自动回退标准 TLS
- TLS 指纹伪装插件重写为独立 sidecar TCP 代理进程（CONNECT + utls + /health 端点）
- 修复启动子进程绑定请求 context 导致被 kill 的问题（改用 `context.Background()`）
- 修复健康检查端口错误：plugin.Port → plugin.Port+1（sidecar 类型 health 在 port+1）

### 插件安装流程优化
- 上传 ZIP 后不再自动安装，改为先展示预览（名称/版本/描述/类型/钩子）
- 新增 `POST /plugins/upload`（解析返回预览 + upload_id）
- 新增 `POST /plugins/install`（根据 upload_id 执行实际安装）
- 前端上传后直接加入列表（uploaded 状态），安装按钮在操作区
- 前端上传按钮增加 loading 效果
- 插件市场按钮根据系统配置 `plugin_registry_url` 动态显隐
- 渠道类型插件自动发现：启动后通过 `/.well-known/channel-type` 注册新渠道类型

### 插件注册中心
- 新增 `marketplace_url` / `plugin_registry_url` 配置项，支持远程插件列表 + 一键安装
- 前端渠道类型下拉框从 API 动态获取（不再硬编码），选插件类型时自动填充 base_url

### 模型管理模块
- `model_catalog` 表 + 全量同步逻辑（SaveModels/Delete/UpdateStatus 触发）
- `/v1/models` 端点实现（OpenAI 兼容格式，返回可见模型）
- 管理端 API：`GET /api/models/catalog`、`PUT /api/models/catalog/:id/visibility`
- 前端模型管理页面：左右两列（已选 / 自定义映射）+ 可见性开关
- 渠道模型配置弹窗整合进 Tab 页，去掉弹窗壳
- 跨渠道自定义模型名自动补全：`GET /channels/custom-model-names`
- 模型设置双栏多选模式（已启用/已禁用），支持全选和批量移动
- 新增自定义模型输入功能：手动输入未抓取到的上游模型

### 仪表盘升级
- 后端新增 `hourly_trend`、`top_models`、`top_channels`、`recent_errors` 统计维度
- 前端重写：5 列统计卡片（成功率/延迟颜色规则）+ ECharts 图表 + 异常表格 + 30 秒自动刷新

### Bug 修复
- 修复渠道分组创建后左侧列表统计数不刷新
- 修复密钥分组创建 500 错误（`autoMigrate` 缺少 `KeysGroupChannelGroup` 表注册）
- 修复渠道账号创建时可重复添加相同密钥（新增同渠道下密钥去重检测）
- 修复渠道编辑页面优先级默认值矛盾（min 从 0 改为 1）
- 修复 GitHub Models 等无 `/v1/models` 端点的渠道测试连接 404
- 修复渠道权重/RPM/TPM 提示文案缺失
- 修复模型配置 Tab 全选按钮显示原始 i18n key（`channels.selectAll` → `common.selectAll`）
- 修复搜索无结果时自定义模型输入框被隐藏的问题（自定义模型输入移出搜索结果区，始终显示）
- 修复 auto-complete 一点击文本框就弹出建议列表 → 改为输入内容变更后才查询匹配

## [0.1.1] - 2026-05-08

### 阶段七：扩展与完备
- Anthropic 适配器（Claude Messages API 协议转换 + 流式）
- Gemini 适配器（Google generateContent 协议转换 + 流式）
- 插件系统核心（Sidecar 进程管理 + 钩子调度 + Go SDK + 安装/启动/停止/卸载）
- Docker 多阶段构建 + docker-compose
- README + CHANGELOG

## [0.1.0] - 2026-05-06

### 阶段一至六：核心功能交付
- 项目骨架与目录结构、配置加载（Viper + .env）、存储抽象层（GORM + SQLite）
- 全局加密服务（AES-256-GCM）、系统日志（zap 按日归档）
- HTTP 代理引擎（连接池/超时/流式 SSE 透传）+ OpenAI 适配器
- 账号池核心逻辑（CRUD/优先级/粘性绑定/故障降级/自动探测恢复/解密缓存）
- 密钥管理 + API Key 认证 + RPM/TPM 配额
- 渠道分组 + 密钥分组管理 + 模型存在性过滤 + 分层确定性路由引擎
- 8 个 RESTful 管理 API + Vue 前端 8 个页面（Dashboard/密钥/渠道/分组/统计/日志/插件/设置）
- 模型自动发现（FetchModels + SaveModels）+ 中英文国际化
- 异步日志写入 + 内存实时计数器 + 日聚合调度器
- 前后端联调通过，29 个单元测试