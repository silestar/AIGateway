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
        
    - 密钥管理
        
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
|`/keys`|KeyList|密钥列表|
|`/keys/:id`|ConsumerDetail|密钥详情（统计）|
|`/channels`|ChannelList|渠道列表|
|`/channels/:id`|ChannelDetail|渠道详情（含账号、模型配置）|
|`/groups`|GroupManagement|分组管理（渠道分组、密钥分组）|
|`/stats/overview`|StatsOverview|统计系统总览|
|`/stats/consumer`|StatsConsumer|用户分析（可选密钥ID参数）|
|`/stats/channel`|StatsChannel|渠道分析（可选渠道ID参数）|
|`/logs/requests`|RequestLogs|请求日志查询|
|`/models`|Models|模型管理|
|`/plugins`|PluginMarket|插件管理|
|`/settings`|SystemSettings|系统配置、日志下载|

## 2. 核心页面详细设计

### 2.1 仪表盘 (Dashboard)

**功能**：展示系统整体运行概况，快速了解核心指标。

**数据来源**：`GET /api/stats/dashboard?days=1|7`

**组件结构**：

- **统计卡片行**（5 列 `n-grid`）：
    
    - 今日总请求数、成功率（颜色规则：>95% 绿、>80% 黄、<80% 红）、平均延迟（颜色规则：<2s 绿、<5s 黄、>5s 红）使用后端 `latency_display` 字段（自动换算 ms/s/m）、活跃密钥数、活跃渠道数。
        
    - 数据为 0 时显示占位提示「今日暂无数据」。

- **Token 使用统计**（5 列 `n-grid` 卡片组）：
    
    - 总 Token 数、平均 TPM、平均 TPR、Token 用量前 3 模型（🥇🥈🥉）。
    - 数据来源：`GET /api/stats/token-stats?days=1|7|30`
    - 切换按钮组：[当天, 7天, 30天]

- **请求趋势图**（ECharts 折线图）：
    
    - 双线：成功（绿色）+ 失败（红色），带面积渐变。
    
    - 支持 **当天 / 7天** 切换（`ButtonGroup`）。
    
    - 当天：按小时粒度，X 轴为 HH:00 格式。
    - 7天：按每日粒度，X 轴为日期格式。
    
    - Y 轴请求数。
        
- **模型请求分布**（ECharts 环形饼图）：Top 5 模型占比，含「其他」汇总。
    
- **渠道负载排行**（ECharts 横向柱状图）：Top 10 渠道今日请求数，含渠道名称和渐变色柱。

- **最近异常请求**（`n-data-table`）：最近 5 条 status_code 非 2xx 或延迟超 10s 的记录。
    
    - 列：时间、模型、状态码（红色/黄色）、延迟（红色/黄色）、错误信息。
    
    - 点击行跳转请求日志页，按 `trace_id` 搜索。

**自动刷新**：每 30 秒自动轮询 API 更新全部数据。
    

### 2.2 密钥管理

#### 2.2.1 密钥列表 (KeyList)

**功能**：查看、搜索、新建密钥，管理API密钥。

**组件**：

- 搜索栏：按名称、状态过滤。
    
- 新建按钮 ：弹出创建对话框，输入名称，自动生成密钥（展示一次，后续只显示脱敏标识）。
    
- 列表表格：
    
    - 列：名称、密钥（脱敏，如 `sk-...****`）、状态（active/disabled）、创建时间、所属密钥分组、今日请求数、操作（详情、复制密钥、重置密钥、禁用/启用）。
        
    - **密钥安全复制**：密钥列不渲染明文，显示脱敏标识 + "复制"按钮。点击"复制" → 弹出确认框 → 调用 `POST /api/keys/:id/reveal-key` → 响应明文直接写入剪贴板（`navigator.clipboard.writeText()`），不进入前端状态变量 → 提示"已复制到剪贴板"。
        
    - 操作菜单：详情跳转、复制密钥（同上流程）、重置密钥（需二次确认）、启用/禁用、删除。
        
- API:
    
    - `GET /api/keys?search=&status=`
        
    - `POST /api/keys` { name }
        
    - `PUT /api/keys/:id/reset-key`
        
    - `PATCH /api/keys/:id/status`
        

#### 2.2.2 密钥详情 (ConsumerDetail)

**功能**：查看单个密钥的使用统计。

**组件**：

- 头部：密钥名称、密钥脱敏 + "复制密钥"按钮（调用 `POST /api/keys/:id/reveal-key`，明文直接写剪贴板）、状态标签。
    
- 日期选择器：预设最近7天/30天/自定义。
    
- 用量趋势图：总请求数、成功/失败次数（叠加折线图），按日/周/月切换。
    
- 模型偏好饼图：该用户使用的模型分布。
    
- 错误分布：按错误码统计。
    
- 峰值时段热力图：24小时×7天请求密度。
    
- API: `/api/stats/consumer?key_id=id&start=&end=&granularity=daily`
    

### 2.3 渠道管理

#### 2.3.1 渠道列表 (ChannelList)

**功能**：展示所有渠道，支持搜索、排序、新建、测试、编辑。

**顶部操作栏**：
- 搜索框：支持按名称、ID、类型或模型名称模糊搜索
- 类型筛选下拉
- 排序选项：按权重（默认）、按ID、按响应时间
- 创建渠道按钮

**列表表格列**：
| 列 | 说明 |
|------|------|
| ID | 自增数字 ID |
| 名称 | 渠道名称，账号数>1 时显示多账号图标（👥）+ tooltip "多个账号" |
| 类型 | 品牌 emoji + CSS 圆点（8px 黄色）+ 类型名（OpenAI / OpenAI Response / Anthropic / Google Gemini） |
| 状态 | 启用/禁用标签 + (活跃账号数/总账号数)，全 active 绿色、部分黄色、零红色 |
| 分组 | 所属渠道分组标签（圆角 NTag） |
| 权重 | 数字，值越大越优先 |
| 响应时间 | 最近测试延迟（<500ms 绿、500ms-2s 黄、>2s 红），未测试显示"未测试" |
| 上次测试 | 相对时间（如 3 hours ago），未测试显示"未测试" |
| 操作 | ⚡测试可用性、⏸/▶启用禁用、⋯更多操作 |

**操作按钮详情**：
- ⚡ 测试可用性：调用 `POST /api/channels/:id/test`，完成后弹窗显示耗时和结果
- ⏸/▶ 启用/禁用：切换渠道状态
- ⋯ 更多操作下拉：
  - 编辑渠道：跳转详情页基本信息 Tab
  - 模型测试：独立 ModelTestDialog 弹窗，端点类型选择 + 流式模式开关 + 模型列表（单测/批量） + 状态圆点（黑/绿/红）
  - 获取模型：打开 ModelSelectModal
  - 上游更新：拉取上游模型列表，标记下线模型（红色标签），一键移除
  - 复制渠道：基本信息+模型映射复制，新渠道默认禁用
  - 管理密钥：跳转详情页账号管理 Tab
  - 删除渠道：二次确认后删除

**行悬停**：背景色微亮

- API:
    - `GET /api/channels` (含 search、sort_by、sort_order 参数)
    - `POST /api/channels`
    - `POST /api/channels/:id/test`
    - `POST /api/channels/:id/test-models` (批量测试，支持 endpoint/stream 参数)
    - `POST /api/channels/:id/test-model` (单模型测试)
    - `GET /api/channels/:id/test-endpoints` (获取可用测试端点)
    - `PUT /api/channels/:id/test-model`
    - `POST /api/channels/:id/copy`
    - `PATCH /api/channels/:id/status`
    - `DELETE /api/channels/:id`

#### 2.3.2 渠道详情 (ChannelDetail)

这是一个复合页面，包含三个子Tab：

**Tab 1: 基本信息**

- 编辑渠道名称、Base URL、类型等常规字段。
- 测试模型：指定用于测试渠道可用性的模型，留空则自动取第一个已配置模型。
- 速率限制配置：
  - RPM 限制：数字输入框，0 为不限制
  - TPM 限制：数字输入框，0 为不限制
  - 每日请求上限：数字输入框，0 为不限制（限制值对此渠道下每个账号独立生效）
    

**Tab 2: 模型配置**

- **“获取模型列表”按钮**：点击后显示加载状态，调用 `/api/channels/:id/fetch-models`，返回模型表格。
    
- **模型表格**：
    
    - 列：上游模型ID、自定义别名（内联编辑）、启用开关（`n-switch`）、操作（删除映射）。
        
    - 支持全选/反选，可按前缀快速筛选。
        
    - 保存按钮：批量提交修改到 `PUT /api/channels/:id/models`。
        
- **手动添加模型按钮**：弹出表单，输入上游ID和别名。
    

**Tab 3: 账号管理**

- 列表：账号优先级（可拖拽行排序，使用`vuedraggable`）、密钥（脱敏 `sk-...****` + "复制"按钮）、备注（双击可直接编辑）、状态标签（active/disabled/cooling）、失败次数/上次失败时间、创建时间。列表仅显示脱敏版本（如 `sk-...****`）。**密钥安全复制**：点击"复制" → 确认框 → `POST /api/accounts/:id/reveal-key` → 写入剪贴板 → 不进入前端状态。
    
- 新建账号按钮：弹出表单，输入API Key（明文，提交后后端加密存储）、备注（可选），自动分配优先级（最大+1）。列表仅显示脱敏版本（如 `sk-...****`）。
    
- 操作：编辑（重新输入Key）、双击备注直接编辑、删除、手动启用/禁用。
    
- API:
    
    - `GET /api/channels/:id/accounts`
        
- `POST /api/channels/:id/accounts` { api_key, remark }

- `PUT /api/accounts/:id` (更新key或优先级)

- `PATCH /api/accounts/:id/priority` { priority }

- `PATCH /api/accounts/:id/status` { status }

- `PATCH /api/accounts/:id/remark` { remark }
        

### 2.4 分组管理 (GroupManagement)

**功能**：创建渠道分组和密钥分组，建立授权关系。

**页面布局**：左右两栏或两个Tab。

- **渠道分组** Tab：
    
    - 分组列表（名称、权重）。
        
    - 点击某个分组进入编辑：显示关联的渠道列表（已选渠道显示权重），可添加渠道并设置权重。
        
- **密钥分组** Tab：
    
    - 分组列表（名称、描述）。
        
    - 编辑界面：关联可访问的渠道分组（多选，并设置该密钥组的配额上限 RPM/TPM）。
        
- API:
    
    - CRUD for `channel_groups` 和 `key_groups` 以及关联映射。
        

### 2.5 统计分析

#### 2.5.1 系统总览 (StatsOverview)

类似于简版仪表盘，但提供更丰富的维度：

- 时间范围选择器（最近24h/7d/30d/自定义）。
    
- 系统级指标卡片（总量、成功、失败、延迟、Token消耗）。
    
- 多图表组合：请求趋势、模型分布、渠道请求排行、密钥活跃度排行。
    
- 导出按钮：导出CSV。
    

#### 2.5.2 用户分析 (StatsConsumer)

- 密钥下拉搜索选择框（可清空看全部）。
    
- 选定密钥后，展示该用户的用量趋势、模型偏好、错误分布，与密钥详情页类似，但集成在统计模块中方便对比。
    
- API: `/api/stats/consumer?key_id=...`
    

#### 2.5.3 渠道分析 (StatsChannel)

- 渠道下拉选择框。
    
- 选定渠道后：用量趋势、分模型请求数、账号负载详情表格（各账号请求数、成功率），为调整账号优先级提供直观依据。
    
- API: `/api/stats/channel?channel_id=...`
    

### 2.6 请求日志 (RequestLogs)

**功能**：强大的请求追踪与故障排查工具。

**组件**：

- 筛选栏：密钥（搜索）、模型、渠道、账号、状态（成功/失败）、时间范围、关键词搜索。
    
- 日志表格：
    
    - 列：时间、密钥名称、模型、渠道→账号（简要）、Token数、延迟、状态、操作（展开详情）。
        
    - 状态使用颜色标签。
        
    - 支持分页。
        
- **展开详情抽屉** (`n-drawer`)：
    
    - 完整请求/响应元数据（脱敏内容）。
        
    - **重试链可视化**：以时间线样式展示每次尝试的渠道、账号、错误信息，最终成功项高亮。
        
- 导出按钮：按当前筛选条件导出CSV (HTTP `export=csv`)。
    
- API:
    
    - `GET /api/logs/requests?key_id=&model=&channel_id=&account_id=&status=&start=&end=&page=&page_size=`
        
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
        

### 2.8 模型管理 (Models)

**功能**：管理对外暴露的模型列表，控制 `/v1/models` 端点返回的模型。

**页面布局**：

- 页面标题："模型管理" + 刷新按钮
- 左右两列布局（`n-grid :cols="2"`）

**左列 — 已选模型**：
- 卡片标题"已选模型" + 计数标签
- 模型列表（`n-list`），每项：
  - 蓝色标签显示模型名
  - 引用渠道数
  - 可见性开关（`n-switch`），带 tooltip 提示效果说明

**右列 — 自定义映射模型**：
- 卡片标题"自定义映射模型" + 计数标签
- 模型列表，每项：
  - 橙色标签显示模型名
  - 引用渠道数
  - 可见性开关

**空状态**：无模型时显示引导提示"请先在渠道中配置模型映射"。

**API**：
- `GET /api/models/catalog` — 获取完整目录
- `PUT /api/models/catalog/:id/visibility` — 切换可见性

**数据自动同步**：模型目录从渠道模型映射自动同步，无需手动添加。

### 2.9 系统设置 (SystemSettings)

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