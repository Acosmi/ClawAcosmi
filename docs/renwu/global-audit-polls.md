# polls.ts 全局审计报告

> 审计日期：2026-02-23 | 审计窗口：W-TS-1

## 概览

| 维度 | TS | Go | Rust | 覆盖率 |
|------|----|----|------|--------|
| 文件数 | 1 | 0 | 0 | 0% |
| 总行数 | 69 | 0 | 0 | 0% |

---

## 逐文件对照

| TS 文件 | 行数 | 导出 | Go 对应 | 状态 |
|---------|------|------|---------|------|
| `polls.ts` | 69 | `PollInput` (type), `NormalizedPollInput` (type), `normalizePollInput()`, `normalizePollDurationHours()` | — | ❌ MISSING |

### 功能分析

纯验证/归一化逻辑，无副作用：

- `normalizePollInput()`：验证 question 非空、options ≥ 2、maxSelections 范围、maxOptions 上限、durationHours ≥ 1
- `normalizePollDurationHours()`：将 duration 钳位至 `[1, maxHours]`

---

## 隐藏依赖审计

| # | 类别 | 结果 |
|---|------|------|
| 1 | npm 包黑盒行为 | ✅ 无第三方依赖 |
| 2 | 全局状态/单例 | ✅ 无 |
| 3 | 事件总线/回调链 | ✅ 无 |
| 4 | 环境变量依赖 | ✅ 无 |
| 5 | 文件系统约定 | ✅ 无 |
| 6 | 协议/消息格式 | ✅ 纯类型定义 |
| 7 | 错误处理约定 | ✅ `throw new Error(msg)` |

---

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 |
|----|------|---------|---------|------|--------|
| PO-01 | 功能缺失 | `polls.ts` | — | 投票输入验证/归一化未迁移到 Go | P2 |

---

## 总结

- P0 差异: **0 项**
- P1 差异: **0 项**
- P2 差异: **1 项**（PO-01）
- **模块审计评级**: **C**（完全未覆盖，但模块极小且为纯逻辑）

## 消费方（9 个文件）

`infra/outbound/message.ts`, `discord/send.shared.ts`, `discord/send.outbound.ts`, `web/outbound.ts`, `web/active-listener.ts`, `plugin-sdk/index.ts`, `gateway/server-methods/send.ts`, `channels/plugins/types.core.ts`

> **建议**：polls.ts 仅 69 行纯验证，但被 9 个频道发送模块消费。当 Go 端实现频道发送投票功能时需迁移。当前可安全延迟。
