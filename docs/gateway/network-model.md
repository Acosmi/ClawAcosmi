---
summary: "Gateway、节点和 Canvas 宿主的连接方式"
read_when:
  - 了解 Gateway 网络模型
title: "网络模型（Network Model）"
---

# 网络模型

> [!IMPORTANT]
> **架构状态**：网络绑定由 **Go Gateway**（`backend/internal/gateway/net.go`）实现。

大多数操作通过 Gateway 流转 — 一个常驻运行的 Go 进程，
管理频道连接和 WebSocket 控制面。

## 核心规则

- **每主机一个 Gateway**，管理所有频道连接。如需隔离或容灾，使用独立 profile 和端口运行多实例。详见 [多 Gateway](/gateway/multiple-gateways)。
- **Loopback 优先**：Gateway WS 默认绑定 `ws://127.0.0.1:19001`。向导默认生成 token。非 loopback 绑定（`--bind tailnet`）必须携带 token。
- **节点连接**：通过 LAN、tailnet 或 SSH 连接到 Gateway WS。
- **Canvas 宿主**：HTTP 文件服务器在 `canvasHost.port`（默认 `gateway.port + 4`）提供 `/__openacosmi__/canvas/`。
- **远程访问**：通常通过 SSH 隧道或 tailnet VPN。详见 [远程访问](/gateway/remote) 和 [发现](/gateway/discovery)。
