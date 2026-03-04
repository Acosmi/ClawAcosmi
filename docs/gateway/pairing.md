---
summary: "Gateway 节点配对：审批、token、生命周期"
read_when:
  - 实现节点配对审批
  - 添加 CLI 审批远程节点流程
  - 扩展 Gateway 协议的节点管理
title: "Gateway 配对"
---

# Gateway 节点配对

> [!IMPORTANT]
> **架构状态**：配对由 **Go Gateway**（`backend/internal/gateway/device_pairing.go`）实现。

Gateway 是节点准入的权威。UI（macOS 应用、未来客户端）只是审批/拒绝待处理请求的前端。

**重要**：WS 节点在 `connect` 时使用**设备配对**（角色 `node`）。
`node.pair.*` 是独立的配对存储，不门控 WS 握手。

## 概念

- **待处理请求**：节点请求加入，需要审批。
- **已配对节点**：已审批的节点，持有签发的认证 token。
- **传输**：Gateway WS 端点转发请求但不决定成员资格。

## 配对流程

1. 节点连接到 Gateway WS 并请求配对。
2. Gateway 存储**待处理请求**并发射 `node.pair.requested` 事件。
3. 通过 CLI 或 UI 审批/拒绝请求。
4. 审批后，Gateway 签发**新 token**（重新配对时轮换 token）。
5. 节点使用 token 重新连接，成为"已配对"状态。

待处理请求在 **5 分钟**后自动过期。

## CLI 工作流（无头友好）

```bash
openacosmi nodes pending
openacosmi nodes approve <requestId>
openacosmi nodes reject <requestId>
openacosmi nodes status
openacosmi nodes rename --node <id|name|ip> --name "客厅 iPad"
```

## API 面（Gateway 协议）

事件：

- `node.pair.requested` — 创建新待处理请求时发射。
- `node.pair.resolved` — 请求被审批/拒绝/过期时发射。

方法：

- `node.pair.request` — 创建或复用待处理请求（幂等）。
- `node.pair.list` — 列出待处理 + 已配对节点。
- `node.pair.approve` — 审批并签发 token。
- `node.pair.reject` — 拒绝请求。
- `node.pair.verify` — 验证 `{ nodeId, token }`。

## 存储

配对状态存储在 Gateway state 目录（默认 `~/.openacosmi`）：

- `~/.openacosmi/nodes/paired.json`
- `~/.openacosmi/nodes/pending.json`

覆盖 `OPENACOSMI_STATE_DIR` 时 `nodes/` 目录随之移动。

安全注意：Token 是机密；`paired.json` 应视为敏感文件。
