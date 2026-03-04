---
summary: "Rust CLI + Go Gateway 双二进制架构、组件和客户端流程"
read_when:
  - 了解整体系统架构
  - 使用 Gateway 协议、客户端或传输层
title: "系统架构"
status: active
arch: rust-cli+go-gateway
---

# 系统架构

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**（ADR-001）。
>
> - **Rust CLI**（`openacosmi`）：用户交互、命令解析、TUI 渲染、本地操作
> - **Go Gateway**（`acosmi`）：服务端逻辑、通道适配、Agent 执行、消息路由
> - 两者通过 **WebSocket RPC** 通信

## 概述

- 一个长驻 **Go Gateway** 拥有所有消息通道（WhatsApp via Baileys、Telegram via grammY、Slack、Discord、Signal、iMessage、WebChat、飞书、钉钉、企业微信）。
- 控制面客户端（macOS app、Rust CLI、Web UI、自动化）通过 **WebSocket** 连接到 Gateway（默认绑定 `127.0.0.1:19001`）。
- **Node**（macOS/iOS/Android/headless）也通过 **WebSocket** 连接，声明 `role: node` 并携带设备能力和命令。
- 每主机一个 Gateway；它是唯一打开 WhatsApp 会话的进程。
- **Canvas 宿主**（默认端口 `18793`）提供 Agent 可编辑的 HTML 和 A2UI。

```
Rust CLI (openacosmi) ──── WebSocket RPC ────→ Go Gateway (acosmi)
         |                                              |
    用户交互                                        服务端逻辑
    命令解析（clap derive）                          通道适配器（Go goroutines）
    TUI 渲染（oa-terminal）                         Agent 执行与消息路由
    本地配置操作                                    WebSocket 服务与会话存储
```

## 组件与流程

### Go Gateway（守护进程）

- 维护所有 Provider 连接（通道适配器在 `backend/internal/channels/` 下）。
- 暴露类型化的 WebSocket API（请求、响应、服务器推送事件）。
- 使用 JSON Schema 验证入站帧。
- 发射事件：`agent`、`chat`、`presence`、`health`、`heartbeat`、`cron`。
- 核心代码位于 `backend/internal/gateway/`。

### Rust CLI（客户端）

- 25 个 Cargo crate 组成的工作区（`cli-rust/crates/`）。
- 一个 WebSocket 连接（通过 `oa-gateway-rpc` crate）。
- 发送请求（`health`、`status`、`send`、`agent`、`system-presence`）。
- 订阅事件（`tick`、`agent`、`presence`、`shutdown`）。
- 启动时间 ~5ms，二进制大小 ~4.3MB（macOS arm64）。

### Node（macOS / iOS / Android / headless）

- 连接到**同一 WebSocket 服务器**，声明 `role: node`。
- 在 `connect` 中提供设备身份信息；配对基于设备（角色 `node`），审批存储在设备配对库中。
- 暴露命令如 `canvas.*`、`camera.*`、`screen.record`、`location.get`。

协议详情：[Gateway 协议](/gateway/protocol)

### WebChat

- 静态 UI，使用 Gateway WebSocket API 获取聊天历史和发送消息。
- 远程部署时通过与其他客户端相同的 SSH/Tailscale 隧道连接。

## 连接生命周期（单客户端）

```
Client                    Gateway
  |                          |
  |---- req:connect -------->|
  |<------ res (ok) ---------|   (或 res error + 关闭)
  |   (payload=hello-ok 携带快照: presence + health)
  |                          |
  |<------ event:presence ---|
  |<------ event:tick -------|
  |                          |
  |------- req:agent ------->|
  |<------ res:agent --------|   (ack: {runId,status:"accepted"})
  |<------ event:agent ------|   (流式)
  |<------ res:agent --------|   (最终: {runId,status,summary})
  |                          |
```

## Wire 协议（摘要）

- 传输层：WebSocket，文本帧 + JSON 载荷。
- 第一帧**必须**是 `connect`。
- 握手后：
  - 请求：`{type:"req", id, method, params}` → `{type:"res", id, ok, payload|error}`
  - 事件：`{type:"event", event, payload, seq?, stateVersion?}`
- 设置 `OPENACOSMI_GATEWAY_TOKEN`（或 `--token`）后，`connect.params.auth.token` 必须匹配，否则断开。
- 有副作用的方法（`send`、`agent`）需要幂等键（idempotency key）以安全重试；服务器维护短时去重缓存。
- Node 必须在 `connect` 中包含 `role: "node"` 以及 caps/commands/permissions。

## 配对与本地信任

- 所有 WebSocket 客户端（操作者 + Node）在 `connect` 中包含**设备身份**。
- 新设备 ID 需要配对审批；Gateway 为后续连接签发**设备令牌**。
- **本地**连接（回环或 Gateway 主机自身的 tailnet 地址）可自动审批以保持同主机 UX 流畅。
- **非本地**连接必须签名 `connect.challenge` nonce 并需要显式审批。
- Gateway 认证（`gateway.auth.*`）仍适用于**所有**连接，无论本地或远程。

详情：[Gateway 协议](/gateway/protocol)、[配对](/channels/pairing)、[安全](/gateway/security)

## 协议类型与代码生成

- Go Gateway 中定义协议结构体（`backend/internal/gateway/protocol/`）。
- JSON Schema 从这些定义生成。
- Swift 模型从 JSON Schema 生成（用于 macOS app）。
- Rust CLI 通过 `oa-types` crate 定义兼容的类型。

## 远程访问

- 推荐：Tailscale 或 VPN。
- 替代方案：SSH 隧道

  ```bash
  ssh -N -L 19001:127.0.0.1:19001 user@host
  ```

- 相同的握手 + 认证令牌在隧道上生效。
- 远程部署可启用 TLS + 可选的证书固定。

## 运维概览

- 启动：`make gateway-dev`（开发）或 `openacosmi gateway`（通过 Rust CLI 管理守护进程）。
- 健康检查：WebSocket `health` 请求（也包含在 `hello-ok` 中），或 `openacosmi health`。
- 监管：launchd/systemd 自动重启（通过 `oa-daemon` crate 管理）。

## 不变量

- 每主机恰好一个 Gateway 控制单个 Baileys 会话。
- 握手是强制的；任何非 JSON 或非 connect 的首帧将导致硬关闭。
- 事件不重播；客户端必须在间隙时刷新。

## 代码位置参考

| 组件 | 位置 |
|------|------|
| Rust CLI 入口 | `cli-rust/crates/oa-cli/src/main.rs` |
| Rust CLI 架构 | `cli-rust/ARCHITECTURE.md` |
| Go Gateway 入口 | `backend/cmd/acosmi/main.go` |
| Go Gateway 内部 | `backend/internal/gateway/` |
| 通道适配器 | `backend/internal/channels/` |
| Agent 运行时 | `backend/internal/agents/` |
| WebSocket 协议 | `backend/internal/gateway/protocol/` |
| Rust RPC 客户端 | `cli-rust/crates/oa-gateway-rpc/` |
