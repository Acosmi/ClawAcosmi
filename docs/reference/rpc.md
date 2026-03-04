---
summary: "RPC 适配器：外部 CLI 集成（signal-cli、legacy imsg）和 Gateway 模式"
read_when:
  - 添加或修改外部 CLI 集成
  - 调试 RPC 适配器（signal-cli、imsg）
title: "RPC 适配器"
status: active
arch: rust-cli+go-gateway
---

# RPC 适配器

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - RPC 适配器由 **Go Gateway** 管理（`backend/internal/channels/`）
> - Signal 适配器：`backend/internal/channels/signal/`

OpenAcosmi 通过 JSON-RPC 集成外部 CLI。目前使用两种模式。

## 模式 A：HTTP 守护进程（signal-cli）

- `signal-cli` 作为守护进程运行，提供 HTTP 上的 JSON-RPC。
- 事件流为 SSE（`/api/v1/events`）。
- 健康探测：`/api/v1/check`。
- 当 `channels.signal.autoStart=true` 时，OpenAcosmi 管理其生命周期。

详见 [Signal](/channels/signal) 的设置和端点。

## 模式 B：stdio 子进程（旧版：imsg）

> **注意：** 新的 iMessage 设置请使用 [BlueBubbles](/channels/bluebubbles)。

- OpenAcosmi 将 `imsg rpc` 作为子进程启动（旧版 iMessage 集成）。
- JSON-RPC 通过 stdin/stdout 行分隔（每行一个 JSON 对象）。
- 无 TCP 端口，无守护进程。

核心方法：

- `watch.subscribe` → 通知（`method: "message"`）
- `watch.unsubscribe`
- `send`
- `chats.list`（探测/诊断）

详见 [iMessage](/channels/imessage) 的旧版设置和寻址（推荐 `chat_id`）。

## 适配器指南

- Gateway 管理进程（启动/停止与供应商生命周期绑定）。
- RPC 客户端保持弹性：超时、退出时重启。
- 优先使用稳定 ID（如 `chat_id`）而非显示字符串。
