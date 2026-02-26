# macos/ 全局审计报告

> 审计日期：2026-02-23 | 审计窗口：W-TS-1

## 概览

| 维度 | TS | Go | Rust | 覆盖率 |
|------|----|----|------|--------|
| 文件数 | 3 | 0 | 0 | 0% |
| 总行数 | 343 | 0 | 0 | 0% |

**说明**：`macos/` 包含两个独立的 Node.js 可执行入口脚本，用于 macOS Swift 原生应用中嵌入 Node.js gateway。Go 二进制 `backend/cmd/openacosmi/` 已完全替代此角色。Rust `oa-daemon` 负责 CLI 侧守护进程管理。

---

## 逐文件对照

| TS 文件 | 行数 | 导出 | Go/Rust 对应 | 状态 |
|---------|------|------|-------------|------|
| `gateway-daemon.ts` | 224 | `main()`（入口脚本） | Go `cmd/openacosmi/cmd_gateway.go` | 🔄 REFACTORED |
| `relay.ts` | 82 | `main()`（入口脚本） | Go `cmd/openacosmi/main.go` | 🔄 REFACTORED |
| `relay-smoke.ts` | 37 | `parseRelaySmokeTest`, `runRelaySmokeTest` | — | ⏭️ DEFERRED |

### 差异详述

**gateway-daemon.ts 🔄 REFACTORED**：

- TS 版：macOS Swift 应用通过 `node gateway-daemon.js --port X --bind Y --token Z` 启动内嵌 gateway
- Go 版：`openacosmi gateway` 命令直接启动 gateway 服务，无需 Node.js
- 功能等价：端口/绑定/信号处理/锁/重启循环均已在 Go 端实现
- Bun `Long` polyfill（L37-44）仅服务于 Baileys/WhatsApp，Go 有原生 protobuf

**relay.ts 🔄 REFACTORED**：

- TS 版：macOS Swift 应用的通用 CLI relay 入口，加载 dotenv + buildProgram + parseAsync
- Go 版：Go 二进制直接提供所有 CLI 命令，不需要 relay 入口

**relay-smoke.ts ⏭️ DEFERRED**：

- QR PNG 渲染烟雾测试，仅供 macOS Swift 应用内部验证
- 可推迟至 macOS 原生应用重构时处理

---

## 隐藏依赖审计

| # | 类别 | 结果 |
|---|------|------|
| 1 | npm 包黑盒行为 | ⚠️ `long` npm 包（Bun protobuf polyfill）— Go 无需 |
| 2 | 全局状态/单例 | ⚠️ `globalThis.Long = Long`（Bun 运行时修补）— Go 无需 |
| 3 | 事件总线/回调链 | ⚠️ `process.on('SIGTERM/SIGINT/SIGUSR1')` — Go `cmd_gateway.go` 已等价实现 |
| 4 | 环境变量依赖 | ⚠️ `OPENACOSMI_GATEWAY_PORT/BIND/TOKEN`, `OPENACOSMI_BUNDLED_VERSION` — Go 已覆盖 |
| 5 | 文件系统约定 | ✅ 无直接 fs 操作 |
| 6 | 协议/消息格式 | ✅ 无自定义协议 |
| 7 | 错误处理约定 | ✅ `catch → console.error → process.exit(1)` |

---

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 |
|----|------|---------|---------|------|--------|
| M-01 | 功能缺失 | `relay-smoke.ts` | — | QR 烟雾测试未迁移（macOS Swift 专用） | P3 |

---

## 总结

- P0 差异: **0 项**
- P1 差异: **0 项**
- P2 差异: **0 项**
- P3 差异: **1 项**（M-01）
- **模块审计评级**: **A**（Go 二进制已完全替代 TS 入口角色，仅 smoke-test 推迟）

## 消费方

无外部消费方（`macos/` 为独立入口脚本，无其他模块 import）。

> **建议**：macos/ 已被 Go 二进制完全替代，可标记为已废弃。relay-smoke.ts 记入 deferred-items.md。
