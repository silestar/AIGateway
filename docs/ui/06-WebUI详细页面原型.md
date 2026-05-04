# WebUI 详细页面原型设计

## 1. 整体布局与路由设计

### 1.1 全局布局

```text
┌────────────────────────────────────────────┐
│  顶部导航栏 (Logo + 语言切换 + 用户信息)      │
├──────────┬────────────────────────────────┤
│          │                                │
│ 侧边栏    │       主内容区                  │
│ (菜单)    │     <router-view>              │
│          │                                │
│          │                                │
└──────────┴────────────────────────────────┘
```

- **顶部导航栏**：系统名称、中英文切换按钮、当前管理员信息。
    
- **侧边栏**：可折叠菜单，包含：
    
    - 仪表盘
        
    - 消费者管理
        
    - 渠道管理
        
    - 分组管理
        
    - 统计分析 (子菜单：系统总览、用户分析、渠道分析)
        
    - 请求日志
        
    - 插件市场
        
    - 系统设置
        
- **主内容区**：根据路由动态渲染页面。

### 1.2 前端路由表

|路径|组件|说明|
|---|---|---|
|`/`|Dashboard|仪表盘|
|`/consumers`|ConsumerList|消费者列表|
|`/consumers/:id`|ConsumerDetail|消费者详情（统计）|
|`/channels`|ChannelList|渠道列表|
|`/channels/:id`|ChannelDetail|渠道详情（含账号、模型配置）|
|`/groups`|GroupManagement|分组管理（渠道分组、消费者分组）|
|`/stats/overview`|StatsOverview|统计系统总览|
|`/stats/consumer`|StatsConsumer|用户分析（可选消费者ID参数）|
|`/stats/channel`|StatsChannel|渠道分析（可选渠道ID参数）|
|`/logs/requests`|RequestLogs|请求日志查询|
|`/plugins`|PluginMarket|插件管理|
|`/settings`|SystemSettings|系统配置、日志下载|

## 2. 核心页面详细设计

### 2.1 仪表盘 (Dashboard)

**功能**：展示系统整体运行概况，快速了解核心指标。

**组件结构**：

- **指标卡片行**（使用 Naive UI `n-grid`）：
    
    - 今日总请求数、成功率、平均延迟、活跃消费者数、活跃渠道数。
        
    - 数据实时获取自 `/api/stats/realtime`。
        
- **请求趋势图**（ECharts 折线图）：
    
    - 近24小时每小时的请求量趋势，支持切换7天/30天。
        
    - API: `/api/stats/overview?granularity=hourly&start=...`
        
- **模型请求分布**（饼图）：按模型别名统计今日请求占比。
    
- **渠道负载排行**（柱状图）：各渠道今日请求数 Top 10。
    
- **最近异常**（表格）：最近的失败请求记录简要列表，提供跳转到请求日志的链接。
    

### 2.2 消费者管理

#### 2.2.1 消费者列表 (ConsumerList)

**功能**：查看、搜索、新建消费者，管理API密钥。

**组件**：

- 搜索栏：按名称、状态过滤。
    
- 新建按钮 ：弹出创建对话框，输入名称，自动生成密钥（展示一次，后续只显示脱敏标识）。
    
- 列表表格：
    
    - 列：名称、密钥（脱敏，如 `sk-...****`）、状态（active/disabled）、创建时间、所属消费者分组、今日请求数、操作（详情、重置密钥、禁用/启用）。
        
    - 操作菜单：详情跳转、重置密钥（需二次确认）、启用/禁用、删除。
        
- API:
    
    - `GET /api/consumers?search=&status=`
        
    - `POST /api/consumers` { name }
        
    - `PUT /api/consumers/:id/reset-key`
        
    - `PATCH /api/consumers/:id/status`
        

#### 2.2.2 消费者详情 (ConsumerDetail)

**功能**：查看单个消费者的使用统计。

**组件**：

- 头部：消费者名称、密钥脱敏、状态标签。
    
- 日期选择器：预设最近7天/30天/自定义。
    
- 用量趋势图：总请求数、成功/失败次数（叠加折线图），按日/周/月切换。
    
- 模型偏好饼图：该用户使用的模型分布。
    
- 错误分布：按错误码统计。
    
- 峰值时段热力图：24小时×7天请求密度。
    
- API: `/api/stats/consumer?consumer_id=id&start=&end=&granularity=daily`
    

### 2.3 渠道管理

#### 2.3.1 渠道列表 (ChannelList)

**功能**：展示所有渠道，支持新建、编辑。

**组件**：

- 新建按钮 → 弹出表单：渠道名称、类型（下拉选择openai/gemini等）、Base URL、状态。
    
- 列表卡片或表格：名称、类型、状态、模型数、活跃账号数/总账号数、今日请求数。
    
- 操作：进入详情、启用/禁用、删除。
    
- API:
    
    - `GET /api/channels`
        
    - `POST /api/channels`
        
    - `PATCH /api/channels/:id`
        

#### 2.3.2 渠道详情 (ChannelDetail)

这是一个复合页面，包含三个子Tab：

**Tab 1: 基本信息**

- 编辑渠道名称、Base URL、类型等常规字段。
    

**Tab 2: 模型配置**

- **“获取模型列表”按钮**：点击后显示加载状态，调用 `/api/channels/:id/fetch-models`，返回模型表格。
    
- **模型表格**：
    
    - 列：上游模型ID、自定义别名（内联编辑）、启用开关（`n-switch`）、操作（删除映射）。
        
    - 支持全选/反选，可按前缀快速筛选。
        
    - 保存按钮：批量提交修改到 `PUT /api/channels/:id/models`。
        
- **手动添加模型按钮**：弹出表单，输入上游ID和别名。
    

**Tab 3: 账号管理**

- 列表：账号优先级（可拖拽行排序，使用`vuedraggable`）、状态标签（active/disabled/cooling）、失败次数/上次失败时间、创建时间。
    
- 新建账号按钮：弹出表单，输入API Key（明文，提交后后端加密存储），自动分配优先级（最大+1）。列表仅显示脱敏版本（如 `sk-...****`）。
    
- 操作：编辑（重新输入Key）、删除、手动启用/禁用。
    
- API:
    
    - `GET /api/channels/:id/accounts`
        
    - `POST /api/channels/:id/accounts` { api_key }
        
    - `PUT /api/accounts/:id` (更新key或优先级)
        
    - `PATCH /api/accounts/:id/priority` { priority }
        
    - `PATCH /api/accounts/:id/status` { status }
        

### 2.4 分组管理 (GroupManagement)

**功能**：创建渠道分组和消费者分组，建立授权关系。

**页面布局**：左右两栏或两个Tab。

- **渠道分组** Tab：
    
    - 分组列表（名称、权重）。
        
    - 点击某个分组进入编辑：显示关联的渠道列表（已选渠道显示权重），可添加渠道并设置权重。
        
- **消费者分组** Tab：
    
    - 分组列表（名称、描述）。
        
    - 编辑界面：关联可访问的渠道分组（多选，并设置该消费者组的配额上限 RPM/TPM）。
        
- API:
    
    - CRUD for `channel_groups` 和 `consumer_groups` 以及关联映射。
        

### 2.5 统计分析

#### 2.5.1 系统总览 (StatsOverview)

类似于简版仪表盘，但提供更丰富的维度：

- 时间范围选择器（最近24h/7d/30d/自定义）。
    
- 系统级指标卡片（总量、成功、失败、延迟、Token消耗）。
    
- 多图表组合：请求趋势、模型分布、渠道请求排行、消费者活跃度排行。
    
- 导出按钮：导出CSV。
    

#### 2.5.2 用户分析 (StatsConsumer)

- 消费者下拉搜索选择框（可清空看全部）。
    
- 选定消费者后，展示该用户的用量趋势、模型偏好、错误分布，与消费者详情页类似，但集成在统计模块中方便对比。
    
- API: `/api/stats/consumer?consumer_id=...`
    

#### 2.5.3 渠道分析 (StatsChannel)

- 渠道下拉选择框。
    
- 选定渠道后：用量趋势、分模型请求数、账号负载详情表格（各账号请求数、成功率），为调整账号优先级提供直观依据。
    
- API: `/api/stats/channel?channel_id=...`
    

### 2.6 请求日志 (RequestLogs)

**功能**：强大的请求追踪与故障排查工具。

**组件**：

- 筛选栏：消费者（搜索）、模型、渠道、账号、状态（成功/失败）、时间范围、关键词搜索。
    
- 日志表格：
    
    - 列：时间、消费者名称、模型、渠道→账号（简要）、Token数、延迟、状态、操作（展开详情）。
        
    - 状态使用颜色标签。
        
    - 支持分页。
        
- **展开详情抽屉** (`n-drawer`)：
    
    - 完整请求/响应元数据（脱敏内容）。
        
    - **重试链可视化**：以时间线样式展示每次尝试的渠道、账号、错误信息，最终成功项高亮。
        
- 导出按钮：按当前筛选条件导出CSV (HTTP `export=csv`)。
    
- API:
    
    - `GET /api/logs/requests?consumer_id=&model=&channel_id=&account_id=&status=&start=&end=&page=&page_size=`
        
    - `GET /api/logs/requests/export?...`
        

### 2.7 插件市场 (PluginMarket)

**功能**：管理Sidecar插件。

**组件**：

- 插件卡片列表：显示名称、版本、描述、状态（运行/停止/异常）。
    
- 上传按钮：上传ZIP包。
    
- 卡片操作：启用/禁用按钮（根据状态变化）、配置按钮（弹出根据`config_schema`生成的动态表单）、卸载按钮（需确认）。
    
- 日志查看入口：可打开一个简易终端显示插件stderr输出（未来功能）。
    
- API:
    
    - `GET /api/plugins`
        
    - `POST /api/plugins/upload`
        
    - `POST /api/plugins/:id/toggle` (启用/禁用)
        
    - `PUT /api/plugins/:id/config` { config }
        

### 2.8 系统设置 (SystemSettings)

**功能**：系统级配置与运维。

**组件**：

- **基本配置**：表单展示当前 `config.yaml` 中允许热更新的部分（如日志保留天数、默认语言、探测参数），可修改并保存。
    
- **安全**：显示 `.env` 中敏感配置项的状态（如 SECRET_KEY 是否已设置），但不暴露明文；提供重置 KEY 的按钮（危险操作）。
    
- **系统日志**：提供当前日期的系统日志文件链接下载，或显示最近的日志行（可配置）。
    
- **数据维护**：手动触发日志清理、数据库备份（SQLite下载）等。
    
- API:
    
    - `GET /api/system/config`
        
    - `PUT /api/system/config`
        
    - `GET /api/system/logs/download?date=`
        

## 3. 国际化 (i18n) 集成

- 所有界面文本使用 `vue-i18n` 的 `$t('consumer.name')` 形式，语言文件 `locales/zh-CN.json` 和 `en-US.json` 作为硬编码前置。
    
- 顶部语言切换触发 `locale` 变更，并持久化到 `localStorage`。
    
- 后端 API 错误信息可通过 `Accept-Language` 头返回对应语言（前端统一设置）。
    

## 4. API 调用封装

前端统一使用 Axios 实例，配置：

- `baseURL`: `/api`
    
- 请求拦截器：自动添加 `Accept-Language` 头。
    
- 响应拦截器：统一处理错误响应（如 401 跳转登录，但内部管理界面可简化 Bearer Token 硬编码或通过基本认证？初期可简单采用固定的管理员 Token 置于请求头）。
    
- 所有 API 函数集中在 `src/api/` 目录按模块划分。
    

## 5. 组件库与图表

- 基础组件：Naive UI（表格、表单、对话框、开关、标签、抽屉、栅格等）。
    
- 图表：ECharts（通过 `vue-echarts` 封装）。
    
- 拖拽排序：`vuedraggable` (基于 SortableJS) 用于账号优先级调整。
    
- 日期选择器：Naive UI 的 `n-date-picker` 范围选择。
    

## 6. 响应式考虑

- 管理后台主要为桌面端设计，但需保证在平板横屏可用。侧边栏在窄屏时可折叠或覆盖。
    
- 表格列宽自适应，长文本截断并 Tooltip 显示。
    

---

这份原型设计覆盖了所有管理页面，为前端开发提供了明确的结构和交互指引。