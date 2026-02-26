# process/ 全局审计报告

> 审计日期：2026-02-23 | 审计窗口：W-TS-2

## 概览

| 维度 | TS | Go | Rust | 覆盖率 |
|------|----|----|------|--------|
| 文件数 | 5 | 0 | 0 | 0% |
| 总行数 | 513 | 0 | 0 | 0% |

**说明**：process/ 封装 Node.js `child_process` API，提供进程生成、信号桥接、超时执行和 lane-based 命令序列化队列。这些功能高度绑定 Node.js 运行时，Go 有原生 `os/exec`，Rust 有 `tokio::process`，不需要等价迁移。

---

## 逐文件对照

| TS 文件 | 行数 | 导出 | Go 对应 | 状态 |
|---------|------|------|---------|------|
| `exec.ts` | 159 | `runExec()`, `runCommandWithTimeout()`, `SpawnResult`, `CommandOptions` | Go `os/exec` 原生 | 🔄 REFACTORED |
| `command-queue.ts` | 160 | `enqueueCommand()`, `enqueueCommandInLane()`, `setCommandLaneConcurrency()`, `getQueueSize()`, `getTotalQueueSize()`, `clearCommandLane()` | — | ❌ MISSING |
| `spawn-utils.ts` | 141 | `spawnWithFallback()`, `resolveCommandStdio()`, `formatSpawnError()` | — | ❌ MISSING |
| `child-process-bridge.ts` | 47 | `attachChildProcessBridge()` | — | ❌ MISSING |
| `lanes.ts` | 6 | `CommandLane` (enum: Main/Cron/Subagent/Nested) | — | ❌ MISSING |

### 差异详述

**exec.ts 🔄 REFACTORED**：

- TS `runExec/runCommandWithTimeout` 封装 `child_process.execFile/spawn`
- Go 端使用 `os/exec.Command` 直接实现等价功能（分散在各调用处）
- Windows `.cmd` 解析（L14-30）、npm `NPM_CONFIG_FUND=false` 注入（L91-111）为 TS 独有

**command-queue.ts ❌ MISSING**：

- 核心：lane-based 命令序列化队列，防止并发命令交叉 stdin/stdout
- 模块级 `Map<string, LaneState>` 单例，保持进程内全局状态
- Go 端 auto-reply 流程不需要此限制（Go 原生 goroutine 隔离更好）
- 但 TS 端 15+ 个文件 import 此模块，属于关键运行时基础设施

**spawn-utils.ts ❌ MISSING**：

- `spawnWithFallback`：EBADF 重试 + fallback spawn options 链
- 仅在 TS Node.js 运行时有意义

**child-process-bridge.ts ❌ MISSING**：

- 父进程信号 → 子进程信号转发桥
- Go/Rust 有更优的进程组管理

---

## 隐藏依赖审计

| # | 类别 | 结果 |
|---|------|------|
| 1 | npm 包黑盒行为 | ✅ 仅使用 Node.js 内置 `child_process` |
| 2 | 全局状态/单例 | ⚠️ `command-queue.ts:26` — `const lanes = new Map()` 模块级单例 |
| 3 | 事件总线/回调链 | ⚠️ `process.on(signal)`, `child.on('spawn/error/close')` |
| 4 | 环境变量依赖 | ⚠️ `process.env` 合并（exec.ts:103）、`NPM_CONFIG_FUND` 注入 |
| 5 | 文件系统约定 | ✅ 无 |
| 6 | 协议/消息格式 | ✅ 无 |
| 7 | 错误处理约定 | ✅ Promise reject + error codes (`EBADF`, `EPIPE`) |

---

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 |
|----|------|---------|---------|------|--------|
| PR-01 | 架构差异 | `command-queue.ts` | — | Lane-based 命令队列未迁移 — Go goroutine 隔离替代 | P2 |
| PR-02 | 平台适配 | `exec.ts` L14-30 | — | Windows `.cmd` 命令解析未迁移 | P3 |
| PR-03 | 行为差异 | `exec.ts` L91-111 | — | npm `NPM_CONFIG_FUND=false` 注入未迁移 | P3 |
| PR-04 | 功能缺失 | `spawn-utils.ts` | — | EBADF 重试及 fallback spawn 链 | P3 |
| PR-05 | 功能缺失 | `child-process-bridge.ts` | — | 信号转发桥 | P3 |

---

## 总结

- P0 差异: **0 项**
- P1 差异: **0 项**
- P2 差异: **1 项**（PR-01 命令队列）
- P3 差异: **4 项**
- **模块审计评级**: **B**（Node.js 平台绑定模块，Go/Rust 有原生替代方案，不需要等价迁移）

## 消费方（15+ 个文件）

`infra/bonjour-discovery.ts`, `infra/heartbeat-runner.ts`, `infra/clipboard.ts`, `infra/update-check.ts`, `infra/tailscale.ts`, `infra/update-runner.ts`, `infra/control-ui-assets.ts`, `infra/ports-inspect.ts`, `infra/binaries.ts`, `security/fix.ts`, `security/windows-acl.ts`, `plugins/install.ts`, `plugins/runtime/index.ts` 等

> **建议**：process/ 为 Node.js 平台专属模块。Go 端已通过 `os/exec` 在各调用处内联实现等价功能。不建议创建独立 Go package。command-queue 的 lane 概念在 Go 端由 goroutine + channel 自然替代。
