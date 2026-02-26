# pairing/ 全局审计报告

> 审计日期：2026-02-23 | 审计窗口：W-TS-1

## 概览

| 维度 | TS | Go | Rust | 覆盖率 |
|------|----|----|------|--------|
| 文件数 | 3 | 2 (infra/) | 0 (仅 RPC 引用) | 67% (Go) |
| 总行数 | 523 | 256 | — | 49% |

**说明**：Go 端在 `backend/internal/infra/node_pairing.go` (96L) + `node_pairing_ops.go` (160L) 实现了节点配对操作，但属于 infra 模块而非独立 pairing 包。Rust CLI 仅通过 RPC 调用配对相关接口。

---

## 逐文件对照

| TS 文件 | 行数 | 导出 | Go 对应 | 状态 |
|---------|------|------|---------|------|
| `pairing-store.ts` | 497 | `readChannelAllowFromStore`, `addChannelAllowFromStoreEntry`, `removeChannelAllowFromStoreEntry`, `listChannelPairingRequests`, `upsertChannelPairingRequest`, `approveChannelPairingCode`, `PairingRequest`, `PairingChannel` | `node_pairing.go` + `node_pairing_ops.go` | ⚠️ PARTIAL |
| `pairing-labels.ts` | 6 | `resolvePairingIdLabel` | — | ❌ MISSING |
| `pairing-messages.ts` | 20 | `buildPairingReply` | — | ❌ MISSING |

### 差异详述

**pairing-store.ts ⚠️ PARTIAL**：

- Go `node_pairing_ops.go` 实现了节点级配对操作（approve/list/remove），但面向的是**多节点配对**（node pairing），而非 TS 中的**频道级设备配对**（channel pairing）
- TS 实现使用 `proper-lockfile` npm 包做文件锁，Go 使用不同的锁机制
- TS 的 store 文件路径 `{stateDir}/oauth/{channel}-pairing.json` 在 Go 端路径约定可能不同
- 核心业务逻辑（TTL 过期清理、code 生成、excess 裁剪）需验证 Go 端是否等价

---

## 隐藏依赖审计

| # | 类别 | 结果 |
|---|------|------|
| 1 | npm 包黑盒行为 | ⚠️ 使用 `proper-lockfile`（指数退避重试锁）— Go 无等价 |
| 2 | 全局状态/单例 | ✅ 无模块级状态 |
| 3 | 事件总线/回调链 | ✅ 无事件 |
| 4 | 环境变量依赖 | ⚠️ 通过 `process.env` 传递给 `resolveStateDir`/`resolveOAuthDir` |
| 5 | 文件系统约定 | ⚠️ 原子写入（tmp+rename）、chmod 0o600/0o700、`proper-lockfile` 锁文件 |
| 6 | 协议/消息格式 | ✅ JSON 文件格式 `{version:1, requests:[...]}` |
| 7 | 错误处理约定 | ✅ throw Error / try-catch-ignore |

---

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 |
|----|------|---------|---------|------|--------|
| P-01 | 功能缺失 | `pairing-labels.ts` | — | `resolvePairingIdLabel` 未迁移 | P2 |
| P-02 | 功能缺失 | `pairing-messages.ts` | — | `buildPairingReply` 未迁移 | P2 |
| P-03 | npm 依赖 | `pairing-store.ts` | `node_pairing*.go` | `proper-lockfile` 指数退避重试锁 → Go 需等价实现 | P2 |
| P-04 | 架构差异 | `pairing-store.ts` | `node_pairing*.go` | TS 面向 channel pairing，Go 面向 node pairing，语义不同 | P1 |

---

## 总结

- P0 差异: **0 项**
- P1 差异: **1 项**（P-04 架构语义差异）
- P2 差异: **3 项**
- **模块审计评级**: **C**（Go 部分覆盖但语义层不同，TS 仍为运行时主力）

## 消费方（10 个文件引用 pairing/）

`discord/monitor/`, `security/fix.ts`, `security/audit.ts`, `plugins/runtime/`, `web/inbound/access-control.ts`, `web/auto-reply/`, `cli/pairing-cli.ts`, `line/bot-handlers.ts`, `telegram/bot-native-commands.ts`

> **建议**：pairing/ 为 TS Node.js 运行时专用模块（配合频道插件），不建议迁移 Go/Rust。应记入 deferred-items.md 作为 TS 保留模块。
