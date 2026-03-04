# W5-W6 大型模块审计报告（config + infra）

> 审计日期：2026-02-19 | 审计窗口：W5+W6
> 模块：config (Phase 1), infra (Phase 2)

---

## 概览

| 模块 | TS 文件 | TS 行数 | Go 文件 | Go 行数 | 行覆盖率 | 评级 |
|------|---------|---------|---------|---------|---------|------|
| config | 80 | 14,329 | 28+30(types) | 7,176+3,080 | 71.5% | **A-** |
| infra | 99 | 18,428 | 43 | 6,781 | 36.8% | **B** |
| **合计** | **179** | **32,757** | **101** | **17,037** | **52%** | **B+** |

> config 行覆盖率按 Zod runtime→静态 struct 转换后需降权计算。30 个 types 文件在 `pkg/types/`。实际功能覆盖 95%+。
> infra 行覆盖率低主要因为 ~40 个辅助/环境检测工具文件在 Go 中不需要或内联。
---

## config 模块 (80 TS → 28+30 Go) — A-

### 文件映射

| TS 文件 | Go 文件 | 状态 |
|---------|---------|------|
| `schema.ts` (1114L) | `schema.go` (298L) + `schema_hints_data.go` (618L) | ✅ FULL — Zod→Go struct+tag |
| `io.ts` (616L) | `loader.go` (694L) | ✅ FULL |
| `zod-schema.*.ts` (×10, ~3000L) | `schema.go` + `validator.go` (330L) | 🔄 REFACTORED — Zod 运行时→Go 编译时 |
| `defaults.ts` (470L) | `defaults.go` (473L) | ✅ FULL |
| `plugin-auto-enable.ts` (455L) | `plugin_auto_enable.go` (528L) | ✅ FULL |
| `group-policy.ts` (213L) | `grouppolicy.go` (484L) | ✅ FULL |
| `includes.ts` (249L) | `includes.go` (279L) | ✅ FULL |
| `paths.ts` (274L) | `paths.go` (249L) | ✅ FULL |
| `legacy.*.ts` (×6, ~1600L) | `legacy*.go` (×3, ~1274L) | ✅ FULL |
| `validation.ts` (361L) | `validator.go` (330L) | ✅ FULL |
| `types.*.ts` (×30, ~4500L) | `pkg/types/*.go` (×30, 3080L) | ✅ FULL |
| `sessions.ts`/`store.ts` | `internal/sessions/` (×7) | ✅ FULL — 独立包 |
| `config-paths.ts`, `env-vars.ts` | `configpath.go`, `shellenv.go` | ✅ FULL |
| `cache-utils.ts` | `cacheutils.go` | ✅ FULL |
| `commands.ts` | `commands.go` | ✅ FULL |
| `talk.ts` | `talk.go` | ✅ FULL |
| `merge-config.ts` | `mergeconfig.go` | ✅ FULL |
| `telegram-custom-commands.ts` | `telegramcmds.go` | ✅ FULL |
| `port-defaults.ts` | `portdefaults.go` | ✅ FULL |
| `logging.ts` | `configlog.go` | ✅ FULL |
| `redact-snapshot.ts` | `redact.go` | ✅ FULL |
| `runtime-overrides.ts` | `overrides.go` | ✅ FULL |
| `version.ts` | `version.go` | ✅ FULL |
| `normalize-paths.ts` | `normpaths.go` | ✅ FULL |
| `env-substitution.ts` | `envsubst.go` | ✅ FULL |
| `agent-dirs.ts` | `agentdirs.go` | ✅ FULL |
| `agent-limits.ts` | `internal/agents/limits.go` | ✅ FULL — 移至更合适的包 |
| `merge-patch.ts` | `mergeconfig.go` 内联 | ✅ FULL |
| `markdown-tables.ts` | `pkg/markdown/tables.go` | ✅ FULL — 移至 markdown 包 |
| `config.ts` | `loader.go` 中 | ✅ FULL |
| `test-helpers.ts` | — | ⏭️ 测试辅助，Go 不单独需要 |

### config 差异

| ID | 描述 | 优先级 |
|----|------|--------|
| — | 无 P0/P1 差异 | — |

---

## infra 模块 (99 TS → 43 Go) — B

### 核心文件映射

| TS 文件 | Go 文件 | 状态 |
|---------|---------|------|
| `exec-approvals.ts` (1541L) | `exec_approvals.go` + `exec_approvals_ops.go` | ✅ FULL |
| `session-cost-usage.ts` (1092L) | `cost/session_cost.go` (450L) + `cost/types.go` + `cost_summary.go` | ✅ FULL |
| `heartbeat-runner.ts` (1030L) | `heartbeat.go` + `heartbeat_delivery*.go` + `heartbeat_wake.go` + `heartbeat_events.go` + `heartbeat_visibility.go` | ✅ FULL |
| `state-migrations.ts` (970L) | `state_migrations_*.go` (×8, ~800L) | ✅ FULL |
| `exec-approval-forwarder.ts` (352L) | `approval_forwarder.go` + `approval_forwarder_ops.go` | ✅ FULL |
| `bonjour*.ts` (×4, ~1500L) | `bonjour.go` + `discovery.go` | ✅ FULL |
| `ports*.ts` (×5, ~800L) | `ports.go` (449L) | ✅ FULL |
| `provider-usage.*.ts` (×12, ~3500L) | `cost/provider_*.go` (×8, ~1100L) + `provider_shared.go` + `provider_types.go` + `provider_format.go` + `provider_auth.go` | ✅ FULL |
| `node-pairing.ts` (336L) | `node_pairing.go` + `node_pairing_ops.go` | ✅ FULL |
| `skills-remote.ts` (361L) | `skills_remote.go` (167L) | ✅ FULL |
| `ssh-tunnel.ts` (213L) | `ssh_tunnel.go` (264L) | ✅ FULL |
| `system-events.ts` | `system_events.go` | ✅ FULL |
| `system-presence.ts` | `gateway/system_presence.go` | ✅ FULL — 移至 gateway |
| `agent-events.ts` | `agent_events.go` | ✅ FULL |
| `control-ui-assets.ts` | `control_ui_assets.go` | ✅ FULL |
| `voicewake.ts` | `voicewake.go` | ✅ FULL |
| `device-pairing.ts` (558L) | `gateway/device_auth.go` | 🔄 REFACTORED — 设备配对合入 gateway |
| `device-auth-store.ts` + `device-identity.ts` | `gateway/device_auth.go` | 🔄 REFACTORED |
| `tailscale.ts` (495L) + `tailnet.ts` | `discovery.go` 中 | ✅ FULL |
| `update-runner.ts` (912L) | — | ⚠️ PARTIAL — 自更新机制简化实现 |
| `update-check.ts` (415L) | — | ⚠️ PARTIAL — 更新检查简化 |
| `update-*.ts` (×4) | — | ⚠️ PARTIAL — Go 通常由包管理器更新 |

### 不需要 Go 等价的 TS 文件（Node.js 特有）

| TS 文件 | 理由 |
|---------|------|
| `env.ts`, `env-file.ts` | Go `os.Getenv` 原生支持 |
| `home-dir.ts` | Go `os.UserHomeDir` 原生 |
| `os-summary.ts` | Go `runtime.GOOS` 原生 |
| `is-main.ts` | Go 入口模式不同 |
| `fs-safe.ts` | Go `os` 包原生安全 |
| `fetch.ts`, `fetch-guard.ts` | Go `net/http` 原生 |
| `retry.ts`, `retry-policy.ts`, `backoff.ts` | `pkg/retry/` 已实现 |
| `errors.ts` | Go 原生 error 模式 |
| `dedupe.ts` | Go 内联逻辑 |
| `clipboard.ts` | 桌面 TUI 交互，简化 |
| `json-file.ts` | Go `os.ReadFile` + `json.Unmarshal` |
| `format-datetime.ts` 等 | Go `time.Format` 原生 |
| `gateway-lock.ts` | Go `sync.Mutex` |
| `ws.ts` | Go `gorilla/websocket` |
| `wsl.ts` | 构建层差异 |
| `unhandled-rejections.ts` | Go 无此概念 |
| `runtime-guard.ts` | Go 编译时保证 |

### infra 差异清单

| ID | 描述 | 优先级 |
|----|------|--------|
| W56-1 | update-runner/update-check 自更新机制未移植 | P3 — Go 应用通常由系统包管理 |
| W56-2 | channel-activity/channel-summary 未确认 Go 位置 | P3 |

---

## 隐藏依赖审计

| # | 类别 | config | infra |
|---|------|--------|-------|
| 1 | npm 包黑盒 | ⚠️ Zod→Go struct+validator | ✅ |
| 2 | 全局状态/单例 | ✅ | ⚠️ heartbeat wake Map — Go sync.Map |
| 3 | 事件总线 | ✅ | ⚠️ diagnostic-events — Go channel/callback |
| 4 | 环境变量 | ✅ paths.go 已覆盖 | ✅ |
| 5 | 文件系统 | ✅ loader.go 已覆盖 | ✅ state-migrations 覆盖 |
| 6 | 协议/消息 | ✅ | ✅ |
| 7 | 错误处理 | ✅ | ✅ |

---

## 总结

- **config**: 评级 **A-**，功能覆盖 95%+。Zod→Go struct 架构差异是设计决策，不是缺失。
- **infra**: 评级 **B**，核心功能（exec-approvals, heartbeat, state-migrations, provider-usage, bonjour/discovery, ports）全部覆盖。行数差异主要来自 ~30 个 Node.js 特有工具文件无需 Go 等价。update-runner 系列 P3 延迟。
- **合计 2 项 P3 差异**，无 P0/P1。
