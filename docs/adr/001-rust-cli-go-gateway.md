# ADR-001: Rust CLI + Go Gateway 架构

- **状态**: 已采纳
- **日期**: 2026-02-25
- **决策者**: 架构审计

## 背景

项目存在三套 CLI 实现：

| 实现 | 完成度 | 启动速度 | 二进制大小 | 测试 |
|------|--------|----------|-----------|------|
| TS CLI | 100% | ~200ms | ~50MB+ | 有 |
| Go CLI (`cmd/openacosmi`) | ~35% (18 实现 + 68 stub) | ~10ms | ~15MB | 少量 |
| Rust CLI (`cli-rust`) | ~85% (25 crates) | ~5ms | 4.3MB | 1,289 tests |

Go 端实际有两个二进制：
- `openacosmi` (Go CLI) — CLI 命令行工具，35% 实现
- `acosmi` (Gateway) — WebSocket RPC 服务器，完整实现

三套 CLI 通过相同的 WebSocket RPC 协议与 Go Gateway 通信。

## 决策

采用 **Rust CLI + Go Gateway** 架构，各司其职：

```
Rust CLI (openacosmi) ──── WebSocket RPC ────→ Go Gateway (acosmi)
         |                                              |
    用户交互层                                     服务端业务逻辑
    命令解析                                       channels/pairing/agents
    TUI 渲染                                       消息路由/存储
    本地操作                                       WebSocket 服务
```

### 职责划分

| 层 | 语言 | 职责 |
|----|------|------|
| CLI 层 | Rust | 命令解析、TUI、本地操作、Gateway RPC 客户端 |
| Gateway 服务端 | Go | WebSocket 服务、消息路由、渠道适配器、配对、agent 执行 |

### 弃用的方案

**Go 调度 + Rust 执行** — 不推荐，原因：
- IPC 开销（每次命令需 spawn 子进程 + 序列化）
- Go CLI 仅 35% 完成，补全工作量巨大
- Rust 已有完整调度能力（Clap + tokio）
- 双二进制分发增加部署复杂度
- 两套基础设施维护成本翻倍

## Go CLI 命令迁移策略

Go `cmd/openacosmi/` 中的命令分三类处理：

| 分类 | 命令 | 处理 |
|------|------|------|
| Gateway 启动 | `gateway start` | 保留在 `cmd/acosmi`（已增强 CLI flags） |
| 本地工具 | sandbox, doctor, setup, infra | Rust 已有实现，停止 Go 端开发 |
| RPC 转发 | agent, status, memory, logs | Rust 已有实现，停止 Go 端开发 |
| 68 个 Stub | channels, models, cron, skills... | 不再补全，由 Rust 承担 |

## 后果

### 正面
- 每个组件只有一套实现，维护成本最低
- Rust CLI：5ms 启动、4.3MB 单二进制、原生 TUI、1,289 测试
- Go Gateway：goroutine 适合 WebSocket 并发、完整的 channels/pairing/infra
- WebSocket RPC 协议已验证，无需新增 IPC 机制

### 负面
- Go CLI 已实现的 18 个命令代码将被弃用（但不删除，保留兼容期）
- 需要确保 Rust CLI 覆盖所有 Go CLI 已实现的功能

## 实施

1. Go CLI (`cmd/openacosmi`) 添加运行时弃用警告
2. `cmd/acosmi` (Gateway) 增强为自包含二进制（支持 CLI flags）
3. Makefile 默认构建目标改为 Gateway + Rust CLI
4. Rust CLI 作为 `openacosmi` 的唯一发布二进制
5. Go CLI 代码保留但不再开发新功能
