# 管理 API 接口规范

> 版本：1.1  
> 最后更新：2026-05-08  
> 本文档集中定义 AIGateway 所有管理 API 端点，作为前后端开发的单一真相来源。

---

## 通用约定

- **Base URL**：`/api`
- **内容类型**：`application/json; charset=utf-8`
- **认证**：管理端通过 Header `Authorization: Bearer <admin_token>` 认证（初期可硬编码，后续集成完整管理认证）
- **国际化**：通过 `Accept-Language` 头控制错误信息语言（`zh-CN` / `en-US`）
- **分页**：列表接口统一使用 `?page=1&page_size=20` 参数
- **时间格式**：所有时间字段使用 ISO 8601 格式（`2026-05-04T10:30:00Z`）
- **状态码约定**：
  - `200` 成功
  - `201` 创建成功
  - `400` 请求参数错误
  - `401` 未认证
  - `403` 无权限
  - `404` 资源不存在
  - `409` 资源冲突（如名称重复）
  - `429` 配额超限
  - `500` 服务器内部错误

---

## 1. 密钥管理

### 1.1 密钥列表

**`GET /api/keys`**

查询参数：

| 参数 | 类型 | 必填 | 说明 |
|------|------|:--:|------|
| `search` | string | 否 | 按名称模糊搜索 |
| `status` | string | 否 | 按状态筛选：`active` / `disabled` |
| `page` | int | 否 | 页码，默认 1 |
| `page_size` | int | 否 | 每页条数，默认 20 |

响应：

```json
{
  "data": [
    {
      "id": 1,
      "name": "my-app",
      "api_key_masked": "sk-...****",
      "status": "active",
      "key_groups": ["基础用户组"],
      "today_requests": 1523,
      "created_at": "2026-05-01T08:00:00Z",
      "updated_at": "2026-05-04T10:30:00Z"
    }
  ],
  "total": 45,
  "page": 1,
  "page_size": 20
}
```

---

### 1.2 创建密钥

**`POST /api/keys`**

请求体：

```json
{
  "name": "my-app",
  "key_group_ids": [1, 2]
}
```

响应（201）：

```json
{
  "id": 46,
  "name": "my-app",
  "api_key": "sk-agw-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "api_key_masked": "sk-...****",
  "status": "active",
  "key_groups": ["基础用户组", "高级用户组"],
  "created_at": "2026-05-04T15:00:00Z"
}
```

> ⚠️ `api_key` 仅在创建时返回一次，后续不再返回明文。务必提示用户立即保存。

---

### 1.3 密钥详情

**`GET /api/keys/:id`**

响应：

```json
{
  "id": 1,
  "name": "my-app",
  "api_key_masked": "sk-...****",
  "status": "active",
  "key_groups": [
    {"id": 1, "name": "基础用户组", "quota_rpm": 60, "quota_tpm": 100000}
  ],
  "today_requests": 1523,
  "created_at": "2026-05-01T08:00:00Z",
  "updated_at": "2026-05-04T10:30:00Z"
}
```

---

### 1.4 更新密钥

**`PUT /api/keys/:id`**

请求体：

```json
{
  "name": "my-app-v2",
  "key_group_ids": [1, 3]
}
```

响应：

```json
{
  "id": 1,
  "name": "my-app-v2",
  "status": "active",
  "key_groups": ["基础用户组", "新用户组"],
  "updated_at": "2026-05-04T15:30:00Z"
}
```

---

### 1.5 删除密钥

**`DELETE /api/keys/:id`**

响应（200）：

```json
{
  "message": "密钥已删除"
}
```

---

### 1.6 重置密钥密钥

**`PUT /api/keys/:id/reset-key`**

> ⚠️ 需二次确认。重置后旧密钥立即失效。

响应：

```json
{
  "id": 1,
  "name": "my-app",
  "api_key": "sk-agw-yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy",
  "api_key_masked": "sk-...****",
  "updated_at": "2026-05-04T16:00:00Z"
}
```

---

### 1.7 复制密钥密钥

**`POST /api/keys/:id/reveal-key`**

> **安全机制**：该 API 返回明文密钥，仅用于直接写入剪贴板，前端不得将明文存入状态变量。后端记录审计日志（谁、何时、复制了哪个密钥），并设置频率限制（同一密钥每分钟最多 3 次）。

响应：

```json
{
  "id": 1,
  "name": "my-app",
  "api_key": "sk-agw-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
}
```

---

### 1.8 更新密钥状态

**`PATCH /api/keys/:id/status`**

请求体：

```json
{
  "status": "disabled"
}
```

响应：

```json
{
  "id": 1,
  "status": "disabled",
  "updated_at": "2026-05-04T16:05:00Z"
}
```

---

### 1.8 密钥使用统计

**`GET /api/keys/:id/stats`**

查询参数：

| 参数 | 类型 | 必填 | 说明 |
|------|------|:--:|------|
| `start` | string | 是 | 起始日期 `2026-04-01` |
| `end` | string | 是 | 结束日期 `2026-05-04` |
| `granularity` | string | 否 | `daily`（默认）/ `weekly` / `monthly` |
| `model` | string | 否 | 按模型名筛选 |

响应：

```json
{
  "key_id": 1,
  "consumer_name": "my-app",
  "data": [
    {
      "date": "2026-05-04",
      "total_requests": 320,
      "success_requests": 305,
      "fail_requests": 15,
      "avg_latency_ms": 820.5,
      "total_tokens": 450000
    }
  ],
  "summary": {
    "total_requests": 12500,
    "success_rate": 0.96,
    "total_tokens": 18500000
  }
}
```

---

## 2. 渠道管理

### 2.1 渠道列表

**`GET /api/channels`**

查询参数：

| 参数 | 类型 | 必填 | 说明 |
|------|------|:--:|------|
| `status` | string | 否 | `active` / `disabled` |
| `type` | string | 否 | 渠道类型：`openai` / `claude` / `gemini` |

响应：

```json
{
  "data": [
    {
      "id": 1,
      "name": "OpenAI 高速通道",
      "type": "openai",
      "base_url": "https://api.openai.com",
      "status": "active",
      "model_count": 15,
      "active_accounts": 3,
      "total_accounts": 5,
      "today_requests": 8500,
      "created_at": "2026-05-01T08:00:00Z"
    }
  ],
  "total": 8
}
```

---

### 2.2 创建渠道

**`POST /api/channels`**

请求体：

```json
{
  "name": "OpenAI 高速通道",
  "type": "openai",
  "base_url": "https://api.openai.com",
  "api_key": "sk-proj-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
}
```

> `api_key` 为必填项。创建渠道时会自动创建第一个账号。

`type` 枚举值：

| 值 | 说明 |
|---|------|
| `openai` | OpenAI Chat Completions（兼容 Azure、DeepSeek、Moonshot、Groq 等） |
| `openai-response` | OpenAI Responses API |
| `anthropic` | Anthropic Claude Messages API |
| `gemini` | Google Gemini API |

响应（200）：

```json
{
  "data": {
    "id": 9,
    "name": "OpenAI 高速通道",
    "type": "openai",
    "base_url": "https://api.openai.com",
    "status": "active",
    "created_at": "2026-05-04T15:00:00Z"
  },
  "account_id": 11
}
```

---

### 2.3 测试渠道连接

**`POST /api/channels/test-connection`**

> 在创建渠道前测试 Base URL 和 API Key 是否有效。

请求体：

```json
{
  "type": "openai",
  "base_url": "https://api.openai.com",
  "api_key": "sk-proj-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
}
```

响应（200）：

```json
{
  "success": true
}
```

失败响应（200）：

```json
{
  "success": false,
  "error": "fetch models status 401: ..."
}
```

---

### 2.5 获取渠道已配置模型

**`GET /api/channels/:id/models`**

> 获取指定渠道已保存的模型配置列表（含模型映射）。

响应（200）：

```json
{
  "data": [
    {
      "id": 1,
      "channel_id": 9,
      "display_model_name": "deepseek-ai/deepseek-v4-flash",
      "actual_model_name": "deepseek-ai/deepseek-v4-flash",
      "status": "enabled"
    },
    {
      "id": 2,
      "channel_id": 9,
      "display_model_name": "deepseep-preview",
      "actual_model_name": "deepseek-ai/deepseek-v4-pro",
      "status": "enabled"
    }
  ]
}
```

> `display_model_name === actual_model_name` 表示直连上游模型；不等表示自定义映射别名。

---

### 2.6 渠道详情

**`GET /api/channels/:id`**

响应：

```json
{
  "id": 1,
  "name": "OpenAI 高速通道",
  "type": "openai",
  "base_url": "https://api.openai.com",
  "status": "active",
  "extra_config": {},
  "model_count": 15,
  "active_accounts": 3,
  "total_accounts": 5,
  "created_at": "2026-05-01T08:00:00Z",
  "updated_at": "2026-05-03T12:00:00Z"
}
```

---

### 2.4 更新渠道

**`PUT /api/channels/:id`**

请求体：

```json
{
  "name": "OpenAI 高速通道 v2",
  "base_url": "https://api.openai.com/v2",
  "weight": 10,
  "max_rpm": 60,
  "max_tpm": 100000,
  "max_daily_requests": 1000,
  "extra_config": {"timeout": 30}
}
```

---

### 2.5 删除渠道

**`DELETE /api/channels/:id`**

> ⚠️ 删除渠道前必须先删除其下所有账号，或使用 `?force=true` 级联删除。

---

### 2.6 更新渠道状态

**`PATCH /api/channels/:id/status`**

请求体：

```json
{
  "status": "disabled"
}
```

响应（200）：

```json
{
  "data": {
    "id": 1,
    "status": "disabled"
  }
}
```

---

### 2.7 更新渠道权重

**`PATCH /api/channels/:id/weight`**

> 在渠道列表页直接调整权重值，失焦或回车后提交。权重值越高优先级越高。

请求体：

```json
{
  "weight": 100
}
```

响应（200）：

```json
{
  "data": {
    "id": 1,
    "weight": 100
  }
}
```

---

### 2.8 获取上游模型列表

**`POST /api/channels/:id/fetch-models`**

请求体：

```json
{
  "test_key": ""
}
```

> `test_key` 可选；若不填，系统使用该渠道下优先级最高的 active 账号解密后的 Key。

响应（200）：

```json
{
  "data": [
    {
      "id": "gpt-4o",
      "owned_by": "openai"
    },
    {
      "id": "gpt-4o-mini",
      "owned_by": "openai"
    },
    {
      "id": "claude-3-opus",
      "owned_by": "anthropic"
    }
  ]
}
```

> 按模型 `id` 去重（同一模型在多个账号下只返回一次）。`owned_by` 用于前端按供应商分组展示。

---

### 2.9 保存模型配置

**`PUT /api/channels/:id/models`**

请求体：

```json
{
  "models": [
    {"channel_id": 1, "display_model_name": "gpt-4o", "actual_model_name": "gpt-4o", "status": "enabled"},
    {"channel_id": 1, "display_model_name": "my-fast-model", "actual_model_name": "gpt-4o-mini", "status": "enabled"}
  ]
}
```

> `display_model_name === actual_model_name` 表示直连上游模型；不等表示自定义映射别名。
> 保存时校验：映射的目标模型必须在已选模型中，否则前端提示用户「返回修改」或「自动补齐」。

响应（200）：

```json
{
  "message": "模型配置已保存",
  "configured_count": 2
}
```

---

### 2.10 渠道账号列表

**`GET /api/accounts/channel/:channel_id`**

> 注意：实际路由为 `/api/accounts/channel/:channel_id`，非 `/api/channels/:id/accounts`。

响应（200）：

```json
{
  "data": [
    {
      "id": 10,
      "channel_id": 1,
      "api_key_masked": "sk-...****",
      "remark": "主账号",
      "priority": 1,
      "status": "active",
      "created_at": "2026-05-01T08:00:00Z",
      "updated_at": "2026-05-04T10:30:00Z"
    }
  ]
}
```

---

### 2.11 创建渠道账号

**`POST /api/accounts`**

请求体：

```json
{
  "channel_id": 1,
  "api_key": "sk-proj-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "remark": "主账号"
}
```

> `remark` 可选。

响应（200）：

```json
{
  "data": {
    "id": 11,
    "channel_id": 1,
    "api_key_masked": "sk-...****",
    "priority": 6,
    "status": "active",
    "remark": "主账号",
    "created_at": "2026-05-04T15:30:00Z"
  }
}
```

---

## 3. 账号管理

### 3.1 更新账号

**`PUT /api/accounts/:id`**

请求体：

```json
{
  "api_key": "sk-proj-yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy"
}
```

---

### 3.2 删除账号

**`DELETE /api/accounts/:id`**

---

### 3.3 调整账号优先级

**`PUT /api/accounts/:id/priority`**

请求体：

```json
{
  "priority": 2
}
```

> 修改优先级后，系统自动清除该渠道下所有密钥的 Redis 粘性绑定，让新请求使用新优先级重新选择。

响应：

```json
{
  "id": 10,
  "priority": 2,
  "updated_at": "2026-05-04T16:00:00Z"
}
```

---

### 3.4 更新账号状态

**`PATCH /api/accounts/:id/status`**

请求体：

```json
{
  "status": "disabled"
}
```

> `status` 可选值：`active`（手动恢复）、`disabled`（手动禁用）。手动恢复时重置 `consecutive_failures` 为 0。

---

### 3.5 更新账号备注

**`PATCH /api/accounts/:id/remark`**

请求体：

```json
{
  "remark": "新备注内容"
}
```

响应：

```json
{
  "id": 10,
  "remark": "新备注内容"
}
```

> 备注用于标识账号用途（如"主账号"、"备用账号"等），支持双击列表直接编辑。

---

### 3.6 复制账号密钥

**`POST /api/accounts/:id/reveal-key`**

> **安全机制**：同密钥密钥复制。后端记录审计日志，频率限制同一密钥每分钟最多 3 次。

响应：

```json
{
  "id": 10,
  "channel_id": 1,
  "api_key": "sk-proj-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
}
```

---

## 4. 分组管理

### 4.1 渠道分组列表

**`GET /api/channel-groups`**

响应：

```json
{
  "data": [
    {
      "id": 1,
      "name": "GPT-4 高速组",
      "weight": 100,
      "channels": [
        {"channel_id": 1, "channel_name": "OpenAI 高速", "weight": 80},
        {"channel_id": 3, "channel_name": "OpenAI 备用", "weight": 20}
      ],
      "created_at": "2026-05-01T08:00:00Z"
    }
  ],
  "total": 5
}
```

---

### 4.2 创建渠道分组

**`POST /api/channel-groups`**

请求体：

```json
{
  "name": "GPT-4 高速组",
  "weight": 100,
  "channels": [
    {"channel_id": 1, "weight": 80},
    {"channel_id": 3, "weight": 20}
  ]
}
```

---

### 4.3 更新渠道分组

**`PUT /api/channel-groups/:id`**

请求体同上。

---

### 4.4 删除渠道分组

**`DELETE /api/channel-groups/:id`**

---

### 4.5 密钥分组列表

**`GET /api/consumer-groups`**

响应：

```json
{
  "data": [
    {
      "id": 1,
      "name": "基础用户组",
      "description": "默认分组",
      "consumers": [
        {"key_id": 1, "consumer_name": "my-app", "quota_rpm": 60, "quota_tpm": 100000}
      ],
      "bound_channel_groups": [
        {"channel_group_id": 1, "channel_group_name": "GPT-4 高速组"}
      ],
      "created_at": "2026-05-01T08:00:00Z"
    }
  ],
  "total": 3
}
```

---

### 4.6 创建密钥分组

**`POST /api/consumer-groups`**

请求体：

```json
{
  "name": "基础用户组",
  "description": "默认分组",
  "consumers": [
    {"key_id": 1, "quota_rpm": 60, "quota_tpm": 100000}
  ],
  "channel_group_ids": [1, 2]
}
```

---

### 4.7 更新密钥分组

**`PUT /api/consumer-groups/:id`**

请求体同上。

---

### 4.8 删除密钥分组

**`DELETE /api/consumer-groups/:id`**

---

### 4.9 绑定密钥分组到渠道分组

**`PUT /api/consumer-groups/:id/bindings`**

请求体：

```json
{
  "channel_group_ids": [1, 2, 3]
}
```

---

## 5. 统计分析

### 5.1 系统总览

**`GET /api/stats/overview`**

查询参数：

| 参数 | 类型 | 必填 | 说明 |
|------|------|:--:|------|
| `start` | string | 是 | 起始日期 |
| `end` | string | 是 | 结束日期 |
| `granularity` | string | 否 | `hourly` / `daily`（默认）/ `weekly` / `monthly` |

响应：

```json
{
  "data": [
    {
      "date": "2026-05-04",
      "total_requests": 15230,
      "success_requests": 14800,
      "fail_requests": 430,
      "avg_latency_ms": 750.2,
      "total_tokens": 22500000,
      "unique_consumers": 45,
      "unique_channels": 12
    }
  ],
  "summary": {
    "total_requests": 450000,
    "success_rate": 0.97,
    "total_tokens": 680000000
  }
}
```

---

### 5.2 今日实时指标

**`GET /api/stats/realtime`**

> 数据以 Redis 计数器为准，提供接近实时的当日数据。聚合表仅用于历史查询。

响应：

```json
{
  "date": "2026-05-04",
  "total_requests": 15230,
  "success_requests": 14800,
  "fail_requests": 430,
  "active_consumers": 45,
  "active_channels": 12,
  "updated_at": "2026-05-04T15:30:05Z"
}
```

---

### 5.3 密钥分析

**`GET /api/stats/consumer`**

查询参数：

| 参数 | 类型 | 必填 | 说明 |
|------|------|:--:|------|
| `key_id` | int | 否 | 不填则返回全部密钥汇总 |
| `start` | string | 是 | 起始日期 |
| `end` | string | 是 | 结束日期 |
| `granularity` | string | 否 | `daily`（默认）/ `weekly` |
| `model` | string | 否 | 按模型筛选 |

响应：

```json
{
  "key_id": 5,
  "consumer_name": "my-app",
  "data": [
    {
      "date": "2026-05-04",
      "model_name": "gpt-4o",
      "total_requests": 200,
      "success_requests": 190,
      "fail_requests": 10,
      "avg_latency_ms": 820.0,
      "total_tokens": 350000
    }
  ],
  "summary": {
    "total_requests": 5200,
    "success_rate": 0.95,
    "total_tokens": 9800000,
    "top_models": ["gpt-4o", "gpt-4o-mini", "claude-3-opus"]
  }
}
```

---

### 5.4 渠道分析

**`GET /api/stats/channel`**

查询参数：

| 参数 | 类型 | 必填 | 说明 |
|------|------|:--:|------|
| `channel_id` | int | 否 | 不填则返回全部渠道汇总 |
| `start` | string | 是 | 起始日期 |
| `end` | string | 是 | 结束日期 |
| `granularity` | string | 否 | `daily`（默认）/ `weekly` |

响应：

```json
{
  "channel_id": 3,
  "channel_name": "OpenAI 高速",
  "data": [
    {
      "date": "2026-05-04",
      "model_name": "gpt-4o",
      "total_requests": 5000,
      "success_requests": 4900,
      "fail_requests": 100,
      "avg_latency_ms": 700.5,
      "total_tokens": 8500000,
      "active_accounts": 3
    }
  ],
  "summary": {
    "total_requests": 150000,
    "success_rate": 0.98,
    "total_tokens": 260000000
  }
}
```

---

## 6. 请求日志

### 6.1 请求日志查询

**`GET /api/logs/requests`**

查询参数：

| 参数 | 类型 | 必填 | 说明 |
|------|------|:--:|------|
| `key_id` | int | 否 | 按密钥筛选 |
| `model` | string | 否 | 按模型名筛选 |
| `channel_id` | int | 否 | 按渠道筛选 |
| `account_id` | int | 否 | 按账号筛选 |
| `status` | string | 否 | `success` / `failed` |
| `start` | string | 否 | 起始时间 ISO 8601 |
| `end` | string | 否 | 结束时间 ISO 8601 |
| `search` | string | 否 | 关键词搜索（错误信息等） |
| `page` | int | 否 | 页码，默认 1 |
| `page_size` | int | 否 | 每页条数，默认 20 |

响应：

```json
{
  "data": [
    {
      "id": 12345,
      "timestamp": "2026-05-04T15:30:00Z",
      "key_id": 5,
      "consumer_name": "my-app",
      "model_name": "gpt-4o",
      "channel_id": 3,
      "channel_name": "OpenAI 高速",
      "account_id": 10,
      "is_stream": true,
      "prompt_tokens": 1200,
      "completion_tokens": 800,
      "status_code": 200,
      "error_msg": null,
      "latency_ms": 850,
      "retry_chain_summary": "1次重试后成功"
    }
  ],
  "total": 15230,
  "page": 1,
  "page_size": 20
}
```

---

### 6.2 请求日志详情（重试链）

**`GET /api/logs/requests/:id`**

响应：

```json
{
  "id": 12345,
  "timestamp": "2026-05-04T15:30:00Z",
  "key_id": 5,
  "consumer_name": "my-app",
  "model_name": "gpt-4o",
  "is_stream": true,
  "prompt_tokens": 1200,
  "completion_tokens": 800,
  "status_code": 200,
  "latency_ms": 850,
  "retry_chain": [
    {"channel_id": 3, "account_id": 12, "error": "429"},
    {"channel_id": 3, "account_id": 15, "error": "timeout"},
    {"channel_id": 7, "account_id": 28, "result": "success"}
  ],
  "request_meta": {"model": "gpt-4o", "messages_count": 5},
  "response_meta": {"model": "gpt-4o", "finish_reason": "stop"}
}
```

---

### 6.3 导出请求日志 CSV

**`GET /api/logs/requests/export`**

查询参数：与 6.1 相同（不含分页参数）。

响应：`Content-Type: text/csv`，包含请求日志的 CSV 文件流式下载。

---

## 7. 插件管理

### 7.1 插件列表

**`GET /api/plugins`**

响应：

```json
{
  "data": [
    {
      "id": 1,
      "name": "content-filter",
      "version": "1.0.0",
      "description": "过滤请求中的敏感词",
      "author": "dev@example.com",
      "hooks": ["pre_request"],
      "status": "running",
      "port": 9001,
      "installed_at": "2026-05-03T10:00:00Z"
    }
  ],
  "total": 3
}
```

---

### 7.2 上传插件

**`POST /api/plugins/upload`**

请求：`multipart/form-data`，字段 `plugin` 为 ZIP 文件。

响应（201）：

```json
{
  "id": 4,
  "name": "new-plugin",
  "version": "1.0.0",
  "status": "installed",
  "installed_at": "2026-05-04T16:00:00Z"
}
```

---

### 7.3 启用/禁用插件

**`POST /api/plugins/:id/toggle`**

请求体：

```json
{
  "action": "enable"
}
```

> `action` 可选值：`enable`（启动插件进程）/ `disable`（停止插件进程）

---

### 7.4 更新插件配置

**`PUT /api/plugins/:id/config`**

请求体：

```json
{
  "config": {
    "blocked_words": "word1,word2,word3"
  }
}
```

> 配置内容根据插件的 `config_schema` 动态校验。保存后重启插件进程生效。

---

### 7.5 卸载插件

**`DELETE /api/plugins/:id`**

> 先自动禁用再删除插件目录及数据库记录。

---

## 8. 系统配置

### 8.1 查看系统配置

**`GET /api/system/config`**

响应：

```json
{
  "config": {
    "account_manager": {
      "affinity_ttl": 3600,
      "consecutive_failure_threshold": 5,
      "min_disable_duration": 120,
      "probe_interval": 30,
      "probe_active_ratio_threshold": 0.4,
      "max_probe_failures": 10,
      "probe_cooldown_duration": 7200,
      "max_probe_recover_per_cycle": 1
    },
    "log_retention_days": 365,
    "default_language": "zh-CN",
    "port": 8080
  },
  "security_status": {
    "secret_key_set": true,
    "db_password_set": true,
    "redis_password_set": false
  }
}
```

---

### 8.2 更新系统配置

**`PUT /api/system/config`**

请求体：

```json
{
  "account_manager": {
    "probe_interval": 60,
    "max_probe_recover_per_cycle": 3
  },
  "log_retention_days": 90
}
```

> 仅允许更新可热更新的配置项。敏感配置（如 `SECRET_KEY`）不可通过此接口修改。

响应：

```json
{
  "message": "配置已更新",
  "updated_fields": ["account_manager.probe_interval", "account_manager.max_probe_recover_per_cycle", "log_retention_days"]
}
```

---

### 8.3 下载系统日志

**`GET /api/system/logs/download`**

查询参数：

| 参数 | 类型 | 必填 | 说明 |
|------|------|:--:|------|
| `date` | string | 否 | 日期 `2026-05-04`，不填则下载当天 |

响应：`Content-Type: application/octet-stream`，返回对应日期的日志文件。

---

## 附录 A：错误响应格式

所有错误统一格式：

```json
{
  "error": {
    "code": "CONSUMER_NOT_FOUND",
    "message": "密钥不存在",
    "details": {}
  }
}
```

常见错误码：

| 错误码 | HTTP 状态 | 说明 |
|--------|:--------:|------|
| `VALIDATION_ERROR` | 400 | 请求参数校验失败 |
| `UNAUTHORIZED` | 401 | 未提供有效认证 |
| `FORBIDDEN` | 403 | 无操作权限 |
| `NOT_FOUND` | 404 | 资源不存在 |
| `CONFLICT` | 409 | 资源冲突（名称重复等） |
| `QUOTA_EXCEEDED` | 429 | 密钥配额超限 |
| `INTERNAL_ERROR` | 500 | 服务器内部错误 |

---

## 附录 B：WebSocket 端点（未来扩展）

| 端点 | 说明 |
|------|------|
| `WS /api/ws/dashboard` | 仪表盘实时数据推送（请求数、延迟等指标） |
| `WS /api/ws/logs` | 实时日志流推送 |

> 以上为预留端点，v0.1 版本不实现。

---

本文档将随项目演进持续更新。API 变更须同步更新本文档，确保前后端始终以本文档为单一真相来源。