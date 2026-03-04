---
summary: "使用 SSH 隧道和 Tailnet 进行远程访问"
read_when:
  - 运行或排查远程 Gateway 设置
title: "远程访问"
---

# 远程访问（SSH 隧道与 Tailnet）

> [!IMPORTANT]
> **架构状态**：远程访问由 **Go Gateway** 的绑定模式（`backend/internal/gateway/net.go`）支持。

通过保持单个 Gateway（主节点）在专用宿主机上运行，客户端远程连接。

- **Operator**：SSH 隧道是通用回退。
- **节点**：通过 LAN/tailnet 或 SSH 隧道连接 Gateway WebSocket。

## 核心思路

- Gateway WS 绑定 **loopback**（默认端口 19001）。
- 远程使用通过 SSH 转发端口（或 Tailnet/VPN）。

## 常见 VPN/Tailnet 场景

### 1) Tailnet 中的常驻 Gateway

在持久化宿主机运行 Gateway，通过 Tailscale 或 SSH 访问。

- **最佳体验**：`gateway.bind: "loopback"` + **Tailscale Serve** 暴露控制 UI。
- **回退**：loopback + SSH 隧道。

### 2) 桌面运行 Gateway，笔记本远程控制

笔记本不运行 Agent，远程连接。

### 3) 笔记本运行 Gateway，其他机器远程访问

SSH 隧道或 Tailscale Serve 安全暴露。

## SSH 隧道

```bash
ssh -N -L 19001:127.0.0.1:19001 user@host
```

隧道建立后：

- `openacosmi health` 和 `openacosmi status --deep` 通过 `ws://127.0.0.1:19001` 访问远程 Gateway。
- 替换 `19001` 为配置的 `gateway.port`。
- 使用 `--url` 时需显式提供 `--token`。

## CLI 远程默认值

持久化远程目标：

```json5
{
  gateway: {
    mode: "remote",
    remote: {
      url: "ws://127.0.0.1:19001",
      token: "your-token",
    },
  },
}
```

## 安全规则

- **保持 Gateway loopback-only** 除非确需绑定。
- **非 loopback 绑定**（`lan`/`tailnet`）必须使用认证 token/密码。
- **Tailscale Serve** 可通过身份 header 认证（`gateway.auth.allowTailscale: true`）。

深度阅读：[安全](/gateway/security)。
