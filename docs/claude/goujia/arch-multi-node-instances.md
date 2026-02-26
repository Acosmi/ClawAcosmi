# 多节点互联与实例（Instances）架构说明

## 概述

"实例"（Instances）Tab 显示所有连接到 Gateway 的客户端/节点的在线心跳信息（Presence Beacon）。采用 **Gateway 中心辐射架构**，多台机器通过 WebSocket 连接到同一个 Gateway 实现互联。

---

## 架构模型

```
┌──────────────┐     WebSocket      ┌──────────────────┐
│  机器 B      │ ──────────────────→ │  机器 A (Gateway) │
│  mode:remote │     system-event   │  mode: local      │
└──────────────┘                    │  port: 18789      │
                                    └──────────────────┘
┌──────────────┐     WebSocket             ↑
│  机器 C      │ ──────────────────────────┘
│  mode:remote │
└──────────────┘

Gateway 广播 presence 事件给所有节点 → UI "实例" Tab 显示
```

---

## 配置方式

### 主 Gateway（机器 A） — `openacosmi.json`

```json
{
  "gateway": {
    "mode": "local",
    "port": 18789,
    "bind": "auto",
    "auth": {
      "mode": "token",
      "token": "认证令牌"
    }
  }
}
```

- `bind: "auto"` → 监听 `0.0.0.0:18789` + mDNS 局域网广播
- `bind: "loopback"` → 仅 `127.0.0.1`，不对外

### 远程节点（机器 B） — `openacosmi.json`

```json
{
  "gateway": {
    "mode": "remote",
    "remote": {
      "url": "ws://机器A的IP:18789",
      "transport": "direct",
      "token": "同一个认证令牌"
    }
  }
}
```

也可通过 `openacosmi setup` 引导向导自动发现局域网 Gateway。

---

## 三种连接方式

| 方式 | transport 值 | 说明 |
|------|-------------|------|
| 局域网直连 | `"direct"` | `ws://ip:18789`，最简单 |
| SSH 隧道 | `"ssh"` | 通过 SSH 端口转发，适合跨网络 |
| Tailscale | — | `gateway.tailscale.mode: "serve"` 走 Tailscale 组网 |

---

## 在线心跳机制

### 数据流

```
节点启动 → WebSocket 连接 Gateway → connect RPC（认证）
    → 定期发送 system-event RPC（心跳）
    → Gateway 存储到 SystemPresenceStore
    → 检测变化（host/IP/version/mode）
    → 广播 "presence" 事件给所有 WebSocket 客户端
    → UI "实例" Tab 实时刷新
```

### system-event RPC 载荷

```json
{
  "text": "Node: my-machine (192.168.1.50)",
  "deviceId": "device-uuid",
  "instanceId": "instance-uuid",
  "host": "my-machine",
  "ip": "192.168.1.50",
  "mode": "remote",
  "version": "1.2.3",
  "platform": "darwin",
  "reason": "periodic",
  "lastInputSeconds": 0,
  "roles": ["operator"],
  "scopes": ["operator.admin"]
}
```

### PresenceEntry 字段说明

| 字段 | 含义 |
|------|------|
| `host` / `ip` | 主机名和 IP 地址 |
| `mode` | 角色: `operator`(控制端) / `node`(工作节点) / `device`(设备) |
| `version` | 软件版本 |
| `platform` | 操作系统 (darwin/linux/windows) |
| `deviceFamily` / `modelIdentifier` | 设备类型 (MacBook, iPhone15,2 等) |
| `roles` / `scopes` | 权限角色和授权范围 |
| `lastInputSeconds` | 距上次用户操作的秒数 |
| `reason` | 发送原因: `startup` / `periodic` / `heartbeat` / `offline` |
| `ts` | 时间戳 (毫秒) |

---

## 局域网自动发现 (mDNS/Bonjour)

Gateway 启动时通过 mDNS 广播服务:

- 服务类型: `_openacosmi-gw._tcp`
- macOS 使用 `dns-sd` 命令
- Linux 使用 `avahi-browse` 命令

TXT 记录包含: `role=gateway`, `gatewayPort=18789`, `lanHost=xxx.local`, `displayName=...`

远程节点可自动发现局域网内的 Gateway，无需手动输入 IP。

---

## 关键代码位置

| 文件 | 作用 |
|------|------|
| `backend/internal/gateway/server_methods_system.go` | `system-event` / `system-presence` / heartbeat RPC |
| `backend/internal/gateway/system_presence.go` | SystemPresenceStore 存储 + delta 检测 + 广播 |
| `backend/internal/infra/discovery.go` | mDNS Gateway 发现 |
| `backend/internal/infra/bonjour.go` | mDNS 服务广播 |
| `backend/cmd/openacosmi/setup_remote.go` | 远程连接引导向导 |
| `backend/pkg/types/types_gateway.go` | GatewayConfig / GatewayRemoteConfig 类型 |
| `ui/src/ui/gateway.ts` | WebSocket 客户端协议 |
| `ui/src/ui/views/instances.ts` | 前端实例列表渲染 |
| `ui/src/ui/controllers/presence.ts` | 前端 Presence 数据加载 (system-presence RPC) |

---

## 与"频道"的区别

| 概念 | 说明 |
|------|------|
| **实例 (Instances)** | 运行 OpenAcosmi 的设备/节点的心跳，显示"谁在线" |
| **频道 (Channels)** | 外部消息集成 (飞书/Slack/Discord 等)，显示"哪些平台已接入" |

飞书连接显示在"频道"Tab，不在"实例"Tab。

---

## 单机为空的原因

单机本地模式 (`mode: "local"`) 且无其他节点连接时，实例列表为空属正常状态。只有当第二台机器配置 `mode: "remote"` 并成功连接后，双方才会在"实例"Tab 互相可见。
