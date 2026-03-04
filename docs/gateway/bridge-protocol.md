---
summary: "桥接协议（遗留节点）：TCP JSONL、配对、作用域 RPC"
read_when:
  - 构建或调试节点客户端（iOS/Android/macOS 节点模式）
  - 排查配对或桥接认证失败
title: "桥接协议（遗留）"
---

# 桥接协议（遗留节点传输）

> [!WARNING]
> **架构状态**：桥接协议为**遗留**节点传输（TCP JSONL）。当前 Go Gateway 不再启动 TCP 桥接监听器。
> 所有新客户端应使用统一的 [Gateway WebSocket 协议](/gateway/protocol)。

此文档保留仅供历史参考。遗留 `bridge.*` 配置键已从配置 schema 中移除。

## 为什么曾经存在两种协议

- **安全边界**：桥接暴露小白名单而非完整 Gateway API 面。
- **配对 + 节点身份**：节点准入由 Gateway 管理，绑定到 per-node token。
- **发现 UX**：节点可通过 LAN 上的 Bonjour 发现 Gateway。
- **Loopback WS**：完整 WS 控制面保持本地，除非通过 SSH 隧道。

## 传输层

- TCP，每行一个 JSON 对象（JSONL）。
- 遗留默认端口为 `18790`（当前版本不启动 TCP 桥接）。

## 握手 + 配对

1. 客户端发送 `hello`（含节点元数据 + token）。
2. 未配对时，Gateway 回复 `error`（`NOT_PAIRED`/`UNAUTHORIZED`）。
3. 客户端发送 `pair-request`。
4. Gateway 等待审批，然后发送 `pair-ok` 和 `hello-ok`。

## 帧格式

客户端 → Gateway：

- `req` / `res`：作用域 Gateway RPC
- `event`：节点信号

Gateway → 客户端：

- `invoke` / `invoke-res`：节点命令（`canvas.*`、`camera.*` 等）
- `event`：已订阅会话的聊天更新
- `ping` / `pong`：保活

## 迁移建议

所有新节点客户端应使用 [Gateway WebSocket 协议](/gateway/protocol)，
通过 WebSocket 连接 Go Gateway 并使用 `connect` 握手。
