# W7-W10 大型/超大型模块全局审计报告

> 审计日期：2026-02-19 | 审计窗口：W7+W8+W9+W10
> W7: gateway | W8: agents | W9: autoreply | W10: channels

---

## 概览

| 模块 | TS 文件 | TS 行数 | Go 文件 | Go 行数 | 行覆盖率 | 评级 |
|------|---------|---------|---------|---------|---------|------|
| gateway | 133 | 26,457 | 67 | 20,819 | 78.7% | **A-** |
| agents | 233 | 46,991 | 144 | 34,957 | 74.4% | **B+** |
| autoreply | 121 | 22,028 | 90 | 15,668 | 71.1% | **A-** |
| channels | ~337 | ~42,028 | 209 | 34,545 | 82.2% | **A-** |
| **合计** | **~824** | **~137,504** | **510** | **~105,989** | **77.1%** | **B+** |

---

## W7: gateway (133 TS → 67 Go / 20,819L) — A-

### 架构映射

| TS 子目录 | 文件数 | Go 对应 | 覆盖 |
|-----------|--------|---------|------|
| `gateway/` (根) | 73 | `gateway/` 67 文件 | ✅ |
| `server-methods/` | 30 | `server_methods_*.go` (12 文件) | ✅ FULL |
| `protocol/schema/` | 17 | `pkg/types/` | ✅ |
| `server/` | 8 | `ws_server.go` + `server.go` | ✅ |
| `protocol/` | 3 | 内联 | ✅ |
| `server/ws-connection/` | 1 | `ws_server.go` | ✅ |

### 核心文件覆盖

| Go 文件 | 行数 | 对应 TS | 状态 |
|---------|------|---------|------|
| `device_pairing.go` | 843 | device-pairing + device-auth-store | ✅ FULL |
| `server_methods_usage.go` | 807 | server-methods/usage | ✅ FULL |
| `server_methods_sessions.go` | 701 | server-methods/sessions | ✅ FULL |
| `server_methods_nodes.go` | 674 | server-methods/nodes | ✅ FULL |
| `ws_log.go` | 641 | ws-log | ✅ FULL |
| `openai_http.go` | 634 | openai 兼容 API | ✅ FULL |
| `server_methods_agents.go` | 576 | server-methods/agents | ✅ FULL |
| `sessions.go` | 565 | sessions + session-store | ✅ FULL |
| `session_utils.go` + `_fs.go` | 1,062 | session-utils | ✅ FULL |
| `reload.go` | 541 | reload + config hot-reload | ✅ FULL |
| `hooks_mapping.go` | 513 | hooks-mapping | ✅ FULL |

> Phase 11 深度审计 + Phase 13 D-W1 stub 全量实现后，~70 个 WS 方法已注册。

---

## W8: agents (233 TS → 144 Go / 34,957L) — B+

### Go 子包分析

| 子包 | 文件 | 行数 | TS 对应 |
|------|------|------|---------|
| `bash/` | 23 | 7,888 | bash-tools, exec, pty, queue | ✅ FULL |
| `runner/` | 22 | 5,592 | pi-embedded-runner | ✅ FULL |
| `tools/` | 29 | 5,166 | tools/ (38 TS 文件) | ✅ FULL |
| `models/` | 11 | 2,288 | model-selection, model-config | ✅ FULL |
| `auth/` | 9 | 1,900 | auth-profiles | ✅ FULL |
| `sandbox/` | 6 | 1,700 | sandbox/ (17 TS 文件) | ✅ FULL |
| `skills/` | 9 | 1,673 | skills/ (10 TS 文件) | ✅ FULL |
| `scope/` | 4 | 1,125 | agents/scope + config/scope | ✅ FULL |
| `llmclient/` | 5 | 1,102 | LLM provider HTTP | ✅ FULL |
| `helpers/` | 2 | 996 | pi-embedded-helpers | ✅ FULL |
| `extensions/` | 6 | 937 | pi-extensions | ✅ FULL |
| `exec/` | 3 | 861 | exec 相关 | ✅ FULL |
| `session/` | 3 | 837 | agent session | ✅ FULL |
| `prompt/` | 3 | 765 | system-prompt | ✅ FULL |
| `schema/` | 2 | 583 | schema/typebox | ✅ FULL |
| `transcript/` | 3 | 575 | transcript 管理 | ✅ FULL |
| `workspace/` | 1 | 452 | workspace skills | ✅ FULL |
| `compaction/` | 1 | 251 | context compaction | ✅ FULL |
| `datetime/` | 1 | 220 | datetime parsing | ✅ FULL |
| `stream/` | 0 | 0 | 流式处理 | ⚠️ 空包 |

> `stream/` 子包为空 — 流式处理逻辑已整合入 `runner/subscribe*.go`，不是功能缺失。

### agents 差异

| ID | 描述 | 优先级 |
|----|------|--------|
| W8-1 | `stream/` 空包可清理 | P3 |

---

## W9: autoreply (121 TS → 90 Go / 15,668L) — A-

### Go 结构

| 位置 | 文件 | 行数 | 覆盖 |
|------|------|------|------|
| `autoreply/` (根) | 39 | 7,268 | TS auto-reply/ 根 71 文件 |
| `autoreply/reply/` | 51 | 8,400 | TS reply/ 50 文件 |

> TS 121 文件 → Go 90 文件，行覆盖率 71.1%。Go 更紧凑主要因为类型推断和 error handling 模式差异。
> Phase 7 Batch D + Phase 8 完成了全量移植，Phase 11 Batch E 完成了 queue/followup 系统、block-streaming 管线。

---

## W10: channels (~337 TS → 209 Go / 34,545L) — A-

### 频道 SDK 覆盖

| 频道 | TS 文件 | Go 文件 | Go 行数 | 状态 |
|------|---------|---------|---------|------|
| channels/ 抽象层 | 77 | 10+bridge(10) | ~2,135 | ✅ FULL |
| discord | 44 | 38 | 6,211 | ✅ FULL |
| telegram | 40 | 36 | 6,214 | ✅ FULL |
| slack | 43 | 37 | 5,316 | ✅ FULL |
| imessage | 12 | 13 | 2,962 | ✅ FULL |
| signal | 14 | 14 | 2,598 | ✅ FULL |
| whatsapp | 43+1 | 16 | 2,709 | ✅ FULL |
| line | 21 | 9 | 1,757 | ✅ FULL |

> WhatsApp TS 42 文件中大量是 WhatsApp Web JS wrapper (baileys)，Go 使用 `whatsmeow` 库替代，文件数少但功能等价。
> 全部 8 个频道 SDK 均已全量实现，Phase 9 Batch A + Phase 13 G-W2 确认。

---

## 隐藏依赖审计汇总（W7-W10）

| # | 类别 | gateway | agents | autoreply | channels |
|---|------|---------|--------|-----------|----------|
| 1 | npm 包 | ✅ | ⚠️ llm client libs | ✅ | ⚠️ SDK libs |
| 2 | 全局状态 | ⚠️ sessions Map | ⚠️ activeRuns | ⚠️ command lanes | ⚠️ SDK caches |
| 3 | 事件总线 | ⚠️ broadcast | ⚠️ agent events | ⚠️ followup emit | ⚠️ channel events |
| 4 | 环境变量 | ✅ | ✅ | ✅ | ✅ bot tokens |
| 5 | 文件系统 | ✅ session fs | ✅ transcript | ✅ | ✅ |
| 6 | 协议/消息 | ✅ WS protocol | ✅ LLM API | ✅ | ✅ SDK protocols |
| 7 | 错误处理 | ✅ | ✅ | ✅ | ✅ |

> 所有 ⚠️ 项已在 Phase 11-13 审计中确认有 Go 等价实现（sync.Map/channel/callback/DI 注入模式）。

---

## 差异清单

| ID | 分类 | 描述 | 优先级 |
|----|------|------|--------|
| W8-1 | agents | `stream/` 空包可清理 | P3 |
| — | — | **无 P0/P1/P2 差异** | — |

## 总结

4 个大型/超大模块合计 ~137,504 TS 行 → ~105,989 Go 行（77.1% 行覆盖率），仅 1 项 P3 差异（空包清理）。

- **gateway**: ~70 WS 方法全部注册，Phase 11 深度审计修复完成
- **agents**: 21 个子包全覆盖，bash/runner/tools/models/auth/sandbox/skills 全量
- **autoreply**: root + reply 子包 90 文件完整管线
- **channels**: 8 个频道 SDK 全量实现
