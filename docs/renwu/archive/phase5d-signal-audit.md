# Signal SDK 隐藏依赖深度审计报告

> 审计日期：2026-02-13
> 审计范围：`backend/internal/channels/signal/` 全部 14 个 Go 文件 vs `src/signal/` 14 个 TS 源文件
> 审计方式：逐文件逐函数对照，按 7 类隐藏依赖检查

## 一、总体结论

| 指标 | 结果 |
| ---- | ---- |
| Go 文件总数 | 14 |
| TS 源文件总数 | 14（含 `monitor/event-handler.ts` + `monitor/event-handler.types.ts`） |
| TODO 桩函数 | 5 处（已正确记录于 `deferred-items.md`） |
| 发现隐藏依赖 | **8 处**（3 🔴高 / 4 🟡中 / 1 🟢低） |
| 已修复 | **5 项** ✅（H1/H2/M1/M2/M3） |
| 延迟待办 | **3 项**（H3→Phase 7, M4→Phase 2, L1→Phase 6） |

---

## 二、7 类隐藏依赖逐项审计

### #1 npm 包黑盒行为

⚠️ **存在 2 处**

| # | TS 依赖 | 黑盒行为 | Go 现状 | 状态 |
| - | ------- | ------- | ------ | ---- |
| 1 | `resolveFetch()` from `../infra/fetch.js` | `client.ts:42` 通过 `resolveFetch()` 获取 fetch 实现，内部封装代理检测（`HTTP_PROXY`/`HTTPS_PROXY`）| Go `client.go` 使用标准 `net/http.Client`，默认通过 `http.ProxyFromEnvironment` 支持代理 | 🟢 低风险 — Go 原生代理支持已覆盖 |
| 2 | `computeBackoff` from `../infra/backoff.js` | `sse-reconnect.ts:4` — 指数退避 + jitter 算法 | Go `sse_reconnect.go` 已实现等价算法 | ✅ 无需修复 |

### #2 全局状态/单例

⚠️ **存在 2 处**

| # | 位置 | TS 行为 | Go 现状 | 状态 |
| - | ---- | ------ | ------ | ---- |
| 1 | `monitor.ts:288` | 模块级 Map 缓存群聊历史 | Go `monitor.go` 有对应 `GroupHistories` 字段 | ✅ 无需修复 |
| 2 | `globals.js` → `logVerbose` | 全局标志控制详细日志 | Go 端无 verbose 日志控制 | 🟢 延迟 → Phase 6 日志基础设施 |

### #3 事件总线/回调链

⚠️ **存在 5 处 — 已全部作为 TODO 桩记录**

这 5 处即已知的 TODO(5B) 桩函数（`dispatchInboundMessage`、`enqueueSystemEvent`、`upsertChannelPairingRequest`、`fetchAttachment`、`sendReadReceiptSignal`）。

**验证结果**：`phase5d-task.md` 和 `deferred-items.md` 记录完全吻合 ✅

### #4 环境变量依赖

✅ **无此类依赖** — 所有配置通过 `loadConfig()` → `OpenAcosmiConfig` 对象传入。

### #5 文件系统约定

⚠️ **存在 1 处**

| # | TS 行为 | Go 现状 | 状态 |
| - | ------ | ------ | ---- |
| 1 | `send.ts:124` `saveMediaBuffer("outbound")` + `fetchAttachment` L220 `saveMediaBuffer("inbound")` | Go `send.go` 无媒体保存逻辑 | 🔴 延迟 → Phase 6 媒体存储层（已在 SIG-C 记录） |

### #6 协议/消息格式约定

🔴 **存在 4 处**

| # | 差异 | 影响 | 状态 |
| - | ---- | ---- | ---- |
| 1 | Health Check 端点 `/api/v1/about` 应为 `/api/v1/check` | 接口不兼容 | ✅ **H1 已修复** — `client.go:84` |
| 2 | text-style 序列化 JSON 对象→字符串 `"0:5:BOLD"` | 协议不兼容 | ✅ **H2 已修复** — `send.go` `formatTextStyles()` |
| 3 | JSON-RPC key `textStyle` 应为 `text-style` | 字段名不匹配 | ✅ **H2 已修复**（同上） |
| 4 | Username target `recipient` 应为 `username: [value]` | 参数错误 | ✅ **M1 已修复** — `send.go` `buildTargetParams()` |

### #7 错误处理约定

⚠️ **存在 1 处**

| # | TS 行为 | Go 行为 | 状态 |
| - | ------ | ------ | ---- |
| 1 | HTTP 201 视为成功 | 原 Go 端 201 视为错误 | ✅ **M2 已修复** — `client.go:68` |

---

## 三、修复清单

### ✅ 已修复（5 项）

| # | 文件 | 修复内容 | 修复日期 |
| - | ---- | ------- | ------- |
| H1 | [client.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/channels/signal/client.go) | Health check 端点 `/api/v1/about` → `/api/v1/check` | 2026-02-13 |
| H2 | [send.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/channels/signal/send.go) | text-style 字符串格式 + kebab-case key + 新增 `formatTextStyles()` | 2026-02-13 |
| M1 | [send.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/channels/signal/send.go) | username target → `username: []string{}`；recipient 也改为数组格式 | 2026-02-13 |
| M2 | [client.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/channels/signal/client.go) | HTTP 201 Created 视为成功 | 2026-02-13 |
| M3 | [client.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/channels/signal/client.go) | RPC 超时可配置化（默认 10s 匹配 TS），新增 `SignalRpcRequestWithTimeout()` | 2026-02-13 |

### ⏳ 延迟待办（3 项）

| # | 文件 | 问题 | 延迟至 | 追踪位置 |
| - | ---- | ---- | ------ | ------- |
| H3 | `format.go` 整体 | 使用 regex 而非 Markdown IR 管线，缺少链接处理/样式合并/钳位/表格模式 | Phase 7 | `deferred-items.md` SLK-P7-A |
| M4 | `accounts.go` | 缺少 `normalizeAccountId` 调用 | Phase 2 | P2-D3 routing/session-key |
| L1 | `sse_reconnect.go` | 缺 verbose 日志控制 | Phase 6 | 日志基础设施 |

---

## 四、验证结果

| 检查项 | 结果 |
| ------ | ---- |
| `go build ./...` | ✅ 通过 |
| `go vet ./internal/channels/signal/...` | ✅ 通过 |
| `go test -race ./internal/channels/signal/...` | ✅ 通过（无测试文件） |
| deferred-items Signal TODO 桩 | ✅ 5 处吻合 |
