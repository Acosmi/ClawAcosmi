# Phase 11 模块 A: Gateway 方法 — 审计报告

> 日期: 2026-02-17
> 方法论: `/refactor` 六步循环法 + 隐藏依赖审计

---

## 概述

| 维度 | TS | Go | 差异 |
|------|----|----|------|
| 源文件数（非测试） | 30 | 15 | Go 合并了多个 TS 文件 |
| 总行数 | 7368L | 3442L | Go 为 TS 的 47% |
| 已注册方法数 | ~65 | ~75 | Go 通过 stubs 注册更多 |
| 完整实现方法 | 65 | ~25 | Go 约 38% 完整实现 |
| Stub 方法 | 0 | ~50 | 前端不报错但功能空 |
| 测试文件 | 7 | 7 | Go 测试 2320L |

---

## 方法实现状态总表

### ✅ 已完整实现

| Go 处理器 | 方法名 | 对应 TS | 评估 |
|-----------|--------|---------|------|
| `server_methods_sessions.go` | `sessions.list` | sessions.ts | ✅ 行为对齐 |
| | `sessions.preview` | sessions.ts | ✅ |
| | `sessions.resolve` | sessions.ts | ✅ |
| | `sessions.patch` | sessions.ts | ✅ |
| | `sessions.reset` | sessions.ts | ✅ |
| | `sessions.delete` | sessions.ts | ✅ |
| | `sessions.compact` | sessions.ts | ✅ |
| `server_methods_chat.go` | `chat.history` | chat.ts | ✅ |
| | `chat.abort` | chat.ts | ✅ |
| | `chat.send` | chat.ts | ⚠️ 见差异分析 |
| | `chat.inject` | chat.ts | ✅ |
| `server_methods_system.go` | `system-presence` | system.ts | ✅ |
| | `system-event` | system.ts | ✅ |
| | `last-heartbeat` | system.ts | ✅ |
| | `set-heartbeats` | system.ts | ✅ |
| `server_methods_logs.go` | `logs.tail` | logs.ts | ✅ |
| `server_methods_config.go` | `config.get/set/apply/patch/schema` | config.ts | ✅ |
| `server_methods_exec_approvals.go` | `exec.approvals.get/set` | exec-approvals.ts | ✅ |
| `server_methods_channels.go` | `channels.status` | channels.ts | ⚠️ 简化实现 |
| | `channels.logout` | channels.ts | ✅ DI 回调 |
| `server_methods_agents.go` | `agents.list` | agents.ts | ✅ |
| `server_methods_agent.go` | `agent.identity.get` | agent.ts | ✅ |
| `server_methods_agent_files.go` | `agents.files.list/get/set` | agents.ts | ✅ |
| `server_methods_models.go` | `models.list` | models.ts | ✅ |

### ⚠️ Stub / 骨架实现

| 方法名 | TS 实现规模 | Go 状态 | 优先级 |
|--------|------------|---------|--------|
| `agent` (主处理器) | 388L | ❌ 完全缺失 | **P0** |
| `agent.wait` | 40L | stub（立即返回） | P1 |
| `send` | 200L | 骨架（无 outbound） | P1 |
| `poll` | 120L | stub | P2 |
| `sessions.usage` | 560L | 空数据 stub | P1 |
| `sessions.usage.timeseries` | — | 空数据 stub | P2 |
| `sessions.usage.logs` | — | 空数据 stub | P2 |
| `agents.create/update/delete` | 320L | stub | P1 |
| `exec.approvals.node.get/set` | 80L | stub（需 node registry） | P2 |
| `nodes.*` (11 方法) | 538L | stub | P2 |
| `skills.*` (4 方法) | 216L | stub | P2 |
| `devices.*` (5 方法) | 190L | stub | P2 |
| `cron.*` (7 方法) | 227L | stub | P3 |
| `tts.*` (6 方法) | 157L | stub | P3 |
| `browser.request` | 277L | stub | P3 |
| `wizard.*` (4 方法) | 139L | stub | P3 |
| `web.login.*` (2 方法) | 124L | stub | P3 |

---

## 差异分析 — 关键行为偏差

### DIFF-1: `agent` 主处理器完全缺失 ❌ P0

**TS**: `agent.ts` L46-433 (388L) — 核心 RPC 方法，负责：

- 解析 agent 参数（channel/accountId/sessionKey/text/images）
- Session 自动创建和 key 解析
- 通过 `dispatchInboundMessage` 将用户消息路由到 autoreply 管线
- 发送 `chat.stream.start/update/final` 广播
- Error 处理和 dedupe

**Go**: 完全不存在，未在任何 Handlers 函数中注册，也未在 stubs 中注册。

> [!CAUTION]
> `agent` 方法是前端控制面板「Agent 对话」功能的入口。与 `chat.send` 不同，`agent` 方法绑定特定 agent、支持多频道路由和 session 自动管理。缺失此方法意味着前端无法通过 Agent 面板发送消息。

### DIFF-2: `usage.ts` 822L → 108L (空数据返回) ⚠️ P1

**TS**: 完整的 session discovery + cost aggregation + 模型/供应商/agent/频道/日期维度聚合。
**Go**: schema-correct 空响应（所有 counter 为 0，所有数组为空）。

**影响**: 前端 Usage 页面不崩溃，但无任何实际数据。

### DIFF-3: `agents.ts` CRUD 操作缺失 ⚠️ P1

**TS**: `agents.create` (L193-L275), `agents.update` (L276-L367), `agents.delete` (未导出但存在) — 完整的 agent 配置 CRUD。
**Go**: 仅有 `agents.list`，CRUD 三方法在 stubs 中返回 `{stub: true}`。

**影响**: 前端无法创建/修改/删除 Agent。

### DIFF-4: `send.ts` outbound 管线未接入 ⚠️ P1

**TS**: 完整的频道路由（`getChannelPlugin` → `normalizeChannelId` → plugin outbound）+ `inflightByContext` 去重 + `poll` 轮询。
**Go**: 参数解析后直接返回 `{stub: true, status: "dispatched"}`，无实际消息发送。

### DIFF-5: `channels.status` 简化实现 ⚠️ P2

**TS**: 依赖 `ChannelPlugin` 注册表，动态 probe 频道状态、多账号管理、runtime snapshot。
**Go**: 从 config 静态读取频道配置，硬编码频道列表和标签，无 runtime status。

**影响**: 前端 Channels 页面显示的是配置而非运行时状态。

### DIFF-6: `chat.send` 管线差异 ⚠️ P2

**TS**: `chat.send` L135-695 — 完整管线：timestamp 注入 → image 解析 → session entry 加载 → `dispatchInboundMessage` → `broadcastChatFinal` → dedupe cache。
**Go**: `handleChatSend` 508L — 主体框架已实现（session resolve、abort controller、broadcast），但 `dispatchInboundMessage` 通过 `PipelineDispatcher` DI 回调接入，**实际管线取决于 DI 注入状态**。

**已存在的差异**:

- TS 有 `injectTimestamp` / `timestampOptsFromConfig` — Go 未实现
- TS 有 `parseMessageWithAttachments` (images) — Go 通过 params 直接读取

---

## 隐藏依赖审计 (7 项检查)

| # | 类别 | 结果 | 说明 |
|---|------|------|------|
| 1 | npm 包黑盒 | ✅ | Gateway 方法层无第三方 npm 包直接依赖 |
| 2 | 全局状态/单例 | ⚠️ | TS `costUsageCache` (Map) — Go 未实现；TS `inflightByContext` (WeakMap) — Go 未实现 |
| 3 | 事件总线/回调 | ⚠️ | TS `broadcastChatFinal/Error` 依赖 `context.broadcast` + `nodeSendToSession` — Go 通过 Broadcaster DI 部分实现 |
| 4 | 环境变量 | ✅ | 此层无直接 `process.env` 读取 |
| 5 | 文件系统约定 | ⚠️ | TS usage.ts 依赖磁盘 session discovery（`discoverAllSessionsForUsage`）— Go 全跳过 |
| 6 | 协议/消息格式 | ✅ | 请求/响应 JSON 格式一致 |
| 7 | 错误处理 | ⚠️ | TS `ErrorCodes` 枚举完整（10+ 码）；Go `ErrCode*` 常量覆盖较少 |

---

## 修复优先级和行动计划

### P0 — 必须立即修复

| # | 问题 | 建议 | 预估 |
|---|------|------|------|
| 1 | `agent` 主处理器缺失 | 新建 handler，接入 `dispatchInboundMessage` 管线 | 200-300L |

### P1 — 功能完整性

| # | 问题 | 建议 | 预估 |
|---|------|------|------|
| 2 | `usage` 空数据 | 实现 session discovery + cost 聚合 | 400-500L |
| 3 | `agents` CRUD | 实现 create/update/delete（config 文件操作） | 200-300L |
| 4 | `send` outbound | 接入 channel plugin outbound 管线 | 150-200L |
| 5 | `agent.wait` stub | 接入 agentCommand 等待机制 | 50-80L |

### P2 — 行为对齐

| # | 问题 | 建议 |
|---|------|------|
| 6 | `channels.status` 简化 | 接入 channel plugin runtime probe |
| 7 | `chat.send` timestamp 注入 | 添加 `injectTimestamp` 等价逻辑 |
| 8 | `exec.approvals.node.*` stub | 等 node registry 实现后补全 |

### P3 — 延迟（非核心功能）

nodes, skills, devices, cron, tts, browser, wizard, web.login, talk, voicewake, update — 保持 stub。

---

## 验证

```bash
cd backend && go build ./... && go vet ./... && go test ./internal/gateway/...
```

以上命令已在审计前通过（无编译/代码检查错误）。

---

## 结论

Gateway 方法模块的核心对话管线（`chat.*`, `sessions.*`）实现较为完整，但存在 **1 个 P0 缺失**（`agent` 主处理器）和 **4 个 P1 功能缺口**（usage/agents CRUD/send outbound/agent.wait）。建议在后续窗口中按优先级修复。
