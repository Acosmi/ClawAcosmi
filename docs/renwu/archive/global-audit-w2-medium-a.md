# W2 中型模块 A 全局审计报告

> 审计日期：2026-02-19 | 审计窗口：W2
> 模块：security, routing, sessions, outbound, nodehost

---

## 概览

| 模块 | TS 文件 | TS 行数 | Go 文件 | Go 行数 | 覆盖率 | 评级 |
|------|---------|---------|---------|---------|--------|------|
| security | 8 | 4,028 | 9 | 3,785 | ✅ 100%+ | **A** |
| routing | 3 | 646 | 1(+散布) | 448 | 🔄 95% | **A-** |
| sessions | 6 | 330 | 7 | 1,672 | ✅ 100%+ | **A** |
| outbound | 19 | ~3,500 | 4(+散布) | 1,454 | 🔄 80% | **B+** |
| nodehost | 2 | 1,380 | 13 | 2,795 | ✅ 100%+ | **A** |
| **合计** | **38** | **~9,884** | **34+** | **~10,154** | **~95%** | **A-** |

---

## 逐模块对照

### 1. security (8 TS → 9 Go) ✅ A

| TS 文件 | Go 文件 | 状态 |
|---------|---------|------|
| `audit.ts` | `audit.go` | ✅ FULL |
| `audit-extra.ts` | `audit_extra.go` | ✅ FULL |
| `audit-fs.ts` | `audit_fs.go` | ✅ FULL |
| `channel-metadata.ts` | `channel_metadata.go` | ✅ FULL |
| `external-content.ts` | `external_content.go` | ✅ FULL |
| `fix.ts` | `fix.go` | ✅ FULL |
| `skill-scanner.ts` | `skill_scanner.go` | ✅ FULL |
| `windows-acl.ts` | `windows_acl.go` | ✅ FULL |
| — | `ssrf.go` | ➕ Go 新增 SSRF 防护 |

### 2. routing (3 TS → 1+散布 Go) 🔄 A-

| TS 文件 | Go 位置 | 状态 |
|---------|---------|------|
| `session-key.ts` | `routing/session_key.go` (448L, 18 函数) | ✅ FULL |
| `bindings.ts` | 散布至各频道 `monitor_deps.go` DI 回调 | 🔄 REFACTORED |
| `resolve-route.ts` | 散布至各频道 `monitor_deps.go` + `gateway/` | 🔄 REFACTORED |

> routing 的 bindings/resolve-route 在 Go 中通过 DI 回调模式注入到各频道 monitor，避免循环依赖。核心路由逻辑 `BuildAgentSessionKey` 等函数完整实现。

### 3. sessions (6 TS → 7 Go) ✅ A

| TS 文件 | Go 文件 | 状态 |
|---------|---------|------|
| `level-overrides.ts` | `store.go` 中内联 | ✅ FULL |
| `model-overrides.ts` | `store.go` 中内联 | ✅ FULL |
| `send-policy.ts` | `store.go` 中内联 | ✅ FULL |
| `session-key-utils.ts` | `main_session.go` | ✅ FULL |
| `session-label.ts` | `metadata.go` | ✅ FULL |
| `transcript-events.ts` | `transcript.go` | ✅ FULL |
| — | `paths.go`, `reset.go`, `group.go` | ➕ Go 拆分更细 |

### 4. outbound (19 TS → 4+散布 Go) 🔄 B+

| TS 文件 | Go 位置 | 状态 |
|---------|---------|------|
| `deliver.ts` | `outbound/deliver.go` | ✅ FULL |
| `outbound-policy.ts` | `outbound/policy.go` (292L) | ✅ FULL |
| `outbound-send-service.ts` | `outbound/send.go` | ✅ FULL |
| `outbound-session.ts` | `outbound/session.go` | ✅ FULL |
| `channel-adapters.ts` | `channels/outbound.go` | 🔄 REFACTORED |
| `message-action-runner.ts` | `channels/message_actions.go` | 🔄 REFACTORED |
| `message-action-spec.ts` | `channels/actions.go` | 🔄 REFACTORED |
| `channel-target.ts` | `gateway/server_methods_send.go` | 🔄 REFACTORED |
| `channel-selection.ts` | `channels/outbound.go` | 🔄 REFACTORED |
| `envelope.ts` | `autoreply/reply/` | 🔄 REFACTORED |
| `format.ts` | 内联在各频道 send 文件 | 🔄 REFACTORED |
| `payloads.ts` | `outbound/deliver.go` 内联 | 🔄 REFACTORED |
| `targets.ts` | `outbound/session.go` | 🔄 REFACTORED |
| `target-resolver.ts` | `outbound/session.go` | 🔄 REFACTORED |
| `target-normalization.ts` | `outbound/policy.go` | 🔄 REFACTORED |
| `target-errors.ts` | `outbound/policy.go` | 🔄 REFACTORED |
| `agent-delivery.ts` | `agents/runner/deliver.go` | 🔄 REFACTORED |
| `directory-cache.ts` | DI 注入 | ⚠️ PARTIAL — Map 缓存逻辑简化 |
| `message.ts` | 散布 | ✅ FULL |

> outbound 19 文件 → 4 Go 文件 + 散布到 channels/gateway/autoreply。核心投递管线 `DeliverOutboundPayloads` + 跨上下文安全策略 `EnforceCrossContextPolicy` 完整实现。directory-cache 的 Map TTL 缓存在 Go 中通过 DI 简化。

### 5. nodehost (2 TS → 13 Go) ✅ A

| TS 文件 | Go 文件 | 状态 |
|---------|---------|------|
| `config.ts` | `config.go` | ✅ FULL |
| `runner.ts` | `runner.go` + `invoke.go` + `exec.go` + `types.go` + `sanitize.go` + `skill_bins.go` | ✅ FULL |
| — | `allowlist_*.go` (5 文件) + `browser_proxy.go` | ➕ Go 新增模块 |

---

## 隐藏依赖审计

| # | 类别 | 结果 | 说明 |
|---|------|------|------|
| 1 | npm 包黑盒行为 | ✅ | 无第三方 npm 依赖 |
| 2 | 全局状态/单例 | ⚠️ | outbound `directory-cache` Map + `SLACK_CHANNEL_TYPE_CACHE` — Go 用 DI 接口替代 |
| 3 | 事件总线/回调 | ✅ | 无 EventEmitter |
| 4 | 环境变量 | ⚠️ | security: `DISCORD_BOT_TOKEN` 等检查 — Go `audit_extra.go` 已等价实现 |
| 5 | 文件系统约定 | ✅ | — |
| 6 | 协议/消息格式 | ✅ | — |
| 7 | 错误处理 | ✅ | — |

---

## 差异清单

| ID | 分类 | 描述 | 优先级 |
|----|------|------|--------|
| W2-1 | outbound | `directory-cache.ts` Map+TTL 缓存在 Go 中简化为 DI 注入 | P3 |

## 总结

W2 共 5 个模块审计通过，整体评级 **A-**。仅 1 项 P3 差异（outbound directory-cache 简化）。routing 的 bindings/resolve-route 通过 DI 模式重组属正常架构优化。outbound 虽文件数差异大（19→4），但核心功能 `DeliverOutboundPayloads`、`EnforceCrossContextPolicy` 完整覆盖。
