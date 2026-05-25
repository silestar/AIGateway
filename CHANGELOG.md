# Changelog

AIGateway 版本变更记录。

## [v0.3.3] — 2026-05-25

> **监控与故障恢复机制全面整改第一轮** — 从问题排查到底层冷却逻辑修复 + Settings/SystemMonitor 配置整合。

### 问题排查与修复

- **修复生产数据库 `malformed` 导致消费日志 500** — 宿主机 dump → rebuild → 替换容器内 agw.db，丢失率 0.47%。
- **429 被动熔断缺少冷却时间** — 上游返回 429 时禁用账号未设 `probe_cooldown_until`，导致系统立即尝试探测恢复。现加设一级冷却时间。
- **L2 冷却从未触发** — `consecutive_cooldown_cycles` 始终为 1，`incrementCooldownCycles` 改为原子递增 `cycles+1`，超过 1 次循环自动进入二级冷却。
- **无可用账号渠道仍在重试链中出现** — `Route` 方法遍历前新增 `CountActiveAccountsByChannels` 批量查询，count=0 的渠道静默跳过，减少无效 `no available account` 报错。

### 前端改进

- **账号列表新增诊断列** — 冷却剩余时间、连续失败/阈值、最后禁用时间三列，便于定位故障账号。
- **Settings / SystemMonitor 配置整合** — 原 SystemMonitor 的 9 个账号管理器配置项迁入 Settings 页面，SystemMonitor 改为只读占位（后续迭代完善为实时监控面板）。

### 未落地

- **渠道级熔断降级（P2）** — 生产 Redis 未启用，暂不可用。等 Redis 部署后激活。

### 变更文件

```
internal/account/manager.go     | 23 ++-
internal/account/probe.go       | 16 +-
internal/account/types.go       |  3 +
internal/channel/types.go       |  1 +
internal/group/router.go        |  6 +
web/locales/en-US.json          | 75 +++++--
web/locales/zh-CN.json          | 75 +++++--
web/src/api/account.ts          |  5 +
web/src/views/Channels.vue      | 39 ++++
web/src/views/Settings.vue      | 151 +++++++-
web/src/views/SystemMonitor.vue | 334 +------------------------
11 files changed, 369 insertions(+), 359 deletions(-)
```

## [v0.3.0] — 2026-05-18
- 多数据库适配（SQLite/MySQL/PostgreSQL）
- 插件架构重构（sidecar 钩子系统）
- 数据库迁移工具（支持异构互转）
- 钩子管理弹窗优化与多语言修复

## [v0.2.3] — 2026-05-10
- Dashboard 优化（延迟格式、趋势切换、Token 统计）
- 账号优先级统一全排 + 保留原禁用原因
- 批量恢复异步 + 批量测试下拉菜单 + 进度弹层

## [v0.2.2] — 2026-05-05
- 流式请求路径修复（context.Canceled 处理）
- 代理连接池优化

## [v0.2.1] — 2026-05-01
- FRT 首 Token 时间统计
- 前端国际化修复

## [v0.1.x]
- 初始版本系列