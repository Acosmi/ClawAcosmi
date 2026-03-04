---
summary: "macOS IPC 架构：应用、Gateway 节点传输和 PeekabooBridge"
read_when:
  - 编辑 IPC 协议或菜单栏应用 IPC
title: "macOS IPC"
---

> **架构提示 — Rust CLI + Go Gateway**
> 节点主机通过 Go Gateway WebSocket 通信，
> macOS 应用通过本地 Unix socket 接收 `system.run` 请求。

# OpenAcosmi macOS IPC 架构

**当前模型：** 本地 Unix socket 连接**节点主机服务**到 **macOS 应用**，用于执行审批 + `system.run`。`openacosmi-mac` 调试 CLI 用于发现/连接检查；agent 操作仍通过 Go Gateway WebSocket 和 `node.invoke` 传递。UI 自动化使用 PeekabooBridge。

## 目标

- 单个 GUI 应用实例拥有所有面向 TCC 的工作（通知、屏幕录制、麦克风、语音、AppleScript）。
- 小的自动化接口：Gateway + 节点命令，加上 PeekabooBridge 用于 UI 自动化。
- 可预测的权限：始终相同的签名 bundle ID，由 launchd 启动，TCC 授权持久。

## 工作原理

### Gateway + 节点传输

- 应用运行 Go Gateway（本地模式）并作为节点连接。
- Agent 操作通过 `node.invoke` 执行（如 `system.run`、`system.notify`、`canvas.*`）。

### 节点服务 + 应用 IPC

- 无头节点主机服务连接到 Go Gateway WebSocket。
- `system.run` 请求通过本地 Unix socket 转发到 macOS 应用。
- 应用在 UI 上下文中执行，按需提示，返回输出。

架构图：

```
Agent -> Go Gateway -> 节点服务 (WS)
                          |  IPC (UDS + token + HMAC + TTL)
                          v
                      Mac 应用 (UI + TCC + system.run)
```

### PeekabooBridge（UI 自动化）

- UI 自动化使用名为 `bridge.sock` 的 UNIX socket 和 PeekabooBridge JSON 协议。
- 主机优先级（客户端）：Peekaboo.app → Claude.app → OpenAcosmi.app → 本地执行。
- 安全：bridge 主机需要匹配的 TeamID。

## 操作流程

- 重启/重建：`SIGN_IDENTITY="Apple Development: <Developer Name> (<TEAMID>)" scripts/restart-mac.sh`
- 单实例：如相同 bundle ID 的另一实例运行，应用提前退出。

## 加固说明

- 所有通信保持本地；不暴露网络 socket。
- TCC 提示仅来自 GUI 应用 bundle；保持签名 bundle ID 在重建间稳定。
- IPC 加固：socket 模式 `0600`、token、peer-UID 检查、HMAC 挑战/响应、短 TTL。
