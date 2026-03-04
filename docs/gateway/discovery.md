---
summary: "节点发现与传输方式（Bonjour、Tailscale、SSH）"
read_when:
  - 实现或修改 Bonjour 发现/广播
  - 调整远程连接模式
  - 设计节点发现 + 配对
title: "发现与传输（Discovery）"
---

# 发现与传输

> [!IMPORTANT]
> **架构状态**：发现广播由 **Go Gateway**（`backend/internal/gateway/server_discovery.go`）实现。

OpenAcosmi 有两个不同的问题：

1. **Operator 远程控制**：macOS 菜单栏应用控制运行在其他地方的 Gateway。
2. **节点配对**：iOS/Android 等节点发现 Gateway 并安全配对。

设计目标：所有网络发现/广播保持在 **Go Gateway** 中，客户端作为消费者。

## 术语

- **Gateway**：常驻运行的 Go 进程，拥有状态（会话、配对、节点注册）并运行频道。
- **Gateway WS（控制面）**：默认 `127.0.0.1:19001` 的 WebSocket 端点。
- **Direct WS 传输**：LAN/tailnet 直连 Gateway WS。
- **SSH 传输（回退）**：通过 SSH 转发 `127.0.0.1:19001`。

## 为什么保留 Direct 和 SSH

- **Direct WS** 在同网络和 tailnet 内体验最佳：通过 Bonjour 自动发现、配对 token + ACL 由 Gateway 管理。
- **SSH** 是通用回退：适用于任何有 SSH 访问的场景。

## 发现输入

### 1) Bonjour / mDNS（仅 LAN）

Bonjour 是尽力而为的，不跨网络。仅用于同 LAN 便利。

服务类型：`_openacosmi-gw._tcp`

TXT 键（非机密）：

- `role=gateway`
- `gatewayPort=19001`
- `canvasPort=19005`
- `cliPath=<path>`（可选）
- `tailnetDns=<magicdns>`（可选）

禁用：`OPENACOSMI_DISABLE_BONJOUR=1`

详见：[Bonjour](/gateway/bonjour)

### 2) Tailnet（跨网络）

推荐使用 Tailscale MagicDNS 名称或稳定 tailnet IP。

### 3) 手动 / SSH 目标

无直接路由时，通过 SSH 转发 loopback Gateway 端口。详见 [远程访问](/gateway/remote)。

## 传输选择（客户端策略）

推荐行为：

1. 已配对直连端点可达 → 使用。
2. Bonjour 在 LAN 发现 Gateway → 提供选择并保存。
3. 配置了 tailnet DNS/IP → 尝试直连。
4. 回退到 SSH。

## 配对 + 认证

Gateway 是节点/客户端准入的权威。详见 [Gateway 配对](/gateway/pairing)。

## 各组件职责

- **Gateway**：广播发现信标、管理配对、托管 WS 端点。
- **macOS 应用**：帮助选择 Gateway、显示配对提示、SSH 作为回退。
- **iOS/Android 节点**：浏览 Bonjour、连接到已配对的 Gateway WS。
