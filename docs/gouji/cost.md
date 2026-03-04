# Cost / Provider Usage 架构文档

> 最后更新：2026-02-26 | 代码级审计确认

## 一、模块概述

Cost 模块提供会话级 LLM 成本追踪和多供应商使用量监控。包括 session cost 汇总（JSONL 日志 + 时间序列）和 provider usage API（7 个供应商的实时配额查询 + 格式化输出）。

位于 `internal/infra/cost/`，被 `gateway/server_methods_usage.go` 消费。

## 二、原版实现（TypeScript）

### 源文件列表

| 文件 | 大小 | 职责 |
|------|------|------|
| `provider-usage.types.ts` | ~2KB | 类型定义 |
| `provider-usage.shared.ts` | ~2KB | 共享常量 + 工具函数 |
| `provider-usage.auth.ts` | ~4KB | 认证解析 |
| `provider-usage.fetch.ts` | ~3KB | 路由 + HTTP 客户端 |
| `provider-usage.fetch.claude.ts` | ~6KB | Anthropic 适配器 |
| `provider-usage.fetch.copilot.ts` | ~2KB | GitHub Copilot 适配器 |
| `provider-usage.fetch.gemini.ts` | ~3KB | Gemini 适配器 |
| `provider-usage.fetch.codex.ts` | ~3KB | OpenAI Codex 适配器 |
| `provider-usage.fetch.minimax.ts` | ~12KB | MiniMax 适配器（启发式） |
| `provider-usage.fetch.zai.ts` | ~3KB | z.ai 适配器 |
| `provider-usage.format.ts` | ~4KB | 格式化输出 |
| `provider-usage.load.ts` | ~3KB | 并发加载编排 |

## 三、重构实现（Go）

### 文件结构

| 文件 | 行数 | 对应原版 |
|------|------|----------|
| `provider_types.go` | ~59 | `provider-usage.types.ts` |
| `provider_shared.go` | ~73 | `provider-usage.shared.ts` |
| `provider_auth.go` | ~129 | `provider-usage.auth.ts` |
| `provider_fetch.go` | ~133 | `provider-usage.fetch.ts` + `.load.ts` |
| `provider_fetch_claude.go` | ~107 | `provider-usage.fetch.claude.ts` |
| `provider_fetch_copilot.go` | ~67 | `provider-usage.fetch.copilot.ts` |
| `provider_fetch_gemini.go` | ~90 | `provider-usage.fetch.gemini.ts` |
| `provider_fetch_codex.go` | ~120 | `provider-usage.fetch.codex.ts` |
| `provider_fetch_minimax.go` | ~90 | `provider-usage.fetch.minimax.ts` |
| `provider_fetch_zai.go` | ~80 | `provider-usage.fetch.zai.ts` |
| `provider_format.go` | ~110 | `provider-usage.format.ts` |
| `types.go` | ~60 | session cost 类型 |
| `session_cost.go` | ~150 | session cost 汇总 |
| `cost_summary.go` | ~100 | 聚合摘要 |

### 关键接口

- `UsageProviderId` — 8 个供应商 ID 常量
- `ProviderUsageSnapshot` — 单供应商快照（窗口 + 计划 + 错误）
- `ProviderUsageSummary` — 多供应商聚合视图
- `FetchProviderUsage(ctx, auth)` — 根据认证类型路由到对应适配器
- `LoadProviderUsageSummary(ctx)` — 并发加载所有已配置供应商

### 隐藏依赖审计

| 类别 | 结果 | Go 等价方案 |
|------|------|-------------|
| npm 包黑盒行为 | ✅ | 无 npm 依赖 |
| 全局状态/单例 | ✅ | 无全局状态 |
| 环境变量依赖 | ⚠️ | 8 个 API key 环境变量 — 已对齐 TS |
| 文件系统约定 | ⚠️ | `auth-profiles.json` 读取 — 简化实现 |
| 协议/消息格式 | ⚠️ | 各供应商 API 响应格式 — 已对齐 TS |
| 错误处理约定 | ✅ | 错误降级为 `error` 字段而非 panic |

## 四、延迟项

- **BW1-D1**: 单元测试基础设施（需 `httptest` mock）
- **BW1-D2**: 外部 API 兼容性验证（需真实凭证）
- **BW1-D3**: auth-profiles 正式模块集成

## 五、测试覆盖

| 测试类型 | 状态 | 说明 |
|----------|------|------|
| 编译验证 | ✅ | `go build` 通过 |
| 静态分析 | ✅ | `go vet` 通过 |
| 单元测试 | ⏳ | 延迟 BW1-D1 |
| API 兼容测试 | ⏳ | 延迟 BW1-D2 |
