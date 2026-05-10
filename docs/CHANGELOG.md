# Changelog

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