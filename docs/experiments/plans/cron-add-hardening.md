---
summary: "加固 cron.add 输入处理，对齐 Schema，改进 Cron UI/Agent 工具"
owner: "openacosmi"
status: "complete"
last_updated: "2026-01-05"
title: "Cron 添加加固"
arch: go-gateway
---

# Cron 添加加固与 Schema 对齐

> [!NOTE]
> **架构状态**：Cron 子系统由 **Go Gateway**（`backend/internal/cron/`）实现。
> Schema 验证和负载规范化均在 Go 代码中完成。

## 背景

近期 Gateway 日志显示 `cron.add` 反复失败，参数无效（缺少 `sessionTarget`、`wakeMode`、`payload`，以及格式错误的 `schedule`）。这表明至少有一个客户端（可能是 Agent 工具调用路径）正在发送包装过的或部分指定的作业负载。此外，Go Gateway Schema、CLI 文档和 UI 表单中的 Cron Provider 枚举存在偏差，以及 UI 对 `cron.status` 的显示不匹配（UI 期望 `jobCount`，而 Gateway 返回 `jobs`）。

## 目标

- 通过规范化常见包装负载并推断缺失的 `kind` 字段，消除 `cron.add` 的 INVALID_REQUEST 错误。
- 在 Gateway Schema、Cron 类型定义、CLI 文档和 UI 表单间对齐 Cron Provider 列表。
- 使 Agent Cron 工具 Schema 明确，使 LLM 产生正确的作业负载。
- 修复控制 UI 的 Cron 状态作业计数显示。
- 添加测试覆盖规范化和工具行为。

## 非目标

- 更改 Cron 调度语义或作业执行行为。
- 添加新的调度类型或 Cron 表达式解析。
- 超出必要字段修复范围的 UI/UX 大改造。

## 发现的差距

- Gateway 的 `CronPayloadSchema`（Go struct）中排除了 `signal` + `imessage`，而前端类型定义中包含它们。
- 控制 UI CronStatus 期望 `jobCount`，但 Gateway 返回 `jobs`。
- Agent Cron 工具 Schema 允许任意 `job` 对象，导致格式错误的输入。
- Gateway 对 `cron.add` 严格验证但无规范化，因此包装负载失败。

## 已完成的变更

- `cron.add` 和 `cron.update` 现在规范化常见包装形式并推断缺失的 `kind` 字段。
- Agent Cron 工具 Schema 与 Gateway Schema 保持一致，减少无效负载。
- Provider 枚举在 Gateway、CLI、UI 和 macOS 选择器间已对齐。
- 控制 UI 使用 Gateway 返回的 `jobs` 计数字段显示状态。

## 当前行为

- **规范化**：包装的 `data`/`job` 负载会被解包；在安全的情况下推断 `schedule.kind` 和 `payload.kind`。
- **默认值**：缺失 `wakeMode` 和 `sessionTarget` 时应用安全默认值。
- **Provider**：Discord/Slack/Signal/iMessage 现在在 CLI/UI 中统一呈现。

参见 [定时任务](/automation/cron-jobs) 了解规范化后的结构和示例。

## 验证

- 观察 Gateway 日志，确认 `cron.add` INVALID_REQUEST 错误减少。
- 确认控制 UI 刷新后 Cron 状态显示作业数量。

## 可选后续

- 手动控制 UI 冒烟测试：为每个 Provider 添加一个 Cron 作业 + 验证状态作业计数。

## 待讨论问题

- `cron.add` 是否应接受客户端显式指定的 `state`（当前被 Schema 禁止）？
- 是否应允许 `webchat` 作为显式传递 Provider（当前在传递解析中被过滤）？
