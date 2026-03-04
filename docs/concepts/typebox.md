---
summary: "Gateway 协议 Schema 作为唯一真实来源"
read_when:
  - 更新协议 Schema 或代码生成
title: "协议 Schema"
status: active
arch: go-gateway
---

# Gateway 协议 Schema

> [!IMPORTANT]
> **架构状态**：协议定义由 **Go Gateway** 实现（`backend/internal/gateway/protocol/`）。
> Rust CLI 通过 `oa-types` 和 `oa-gateway-rpc` crate 定义兼容类型。
> 旧版 TypeBox（TypeScript）Schema 已废弃。

## 心智模型

每个 Gateway WebSocket 消息是三种帧之一：

- **请求**：`{ type: "req", id, method, params }`
- **响应**：`{ type: "res", id, ok, payload | error }`
- **事件**：`{ type: "event", event, payload, seq?, stateVersion? }`

第一帧**必须**是 `connect` 请求。之后客户端可调用方法并订阅事件。

## 常用方法与事件

| 类别 | 示例 | 说明 |
|------|------|------|
| 核心 | `connect`、`health`、`status` | `connect` 必须为首帧 |
| 消息 | `send`、`poll`、`agent`、`agent.wait` | 有副作用的需要幂等键 |
| Chat | `chat.history`、`chat.send`、`chat.abort`、`chat.inject` | WebChat 使用 |
| 会话 | `sessions.list`、`sessions.patch`、`sessions.delete` | 会话管理 |
| 节点 | `node.list`、`node.invoke`、`node.pair.*` | 节点操作 |
| 事件 | `tick`、`presence`、`agent`、`chat`、`health`、`shutdown` | 服务器推送 |

## 结构体定义位置

| 组件 | 位置 |
|------|------|
| Go 协议定义 | `backend/internal/gateway/protocol/` |
| Go 服务端处理 | `backend/internal/gateway/server.go` |
| Rust 兼容类型 | `cli-rust/crates/oa-types/src/gateway.rs` |
| Rust RPC 客户端 | `cli-rust/crates/oa-gateway-rpc/` |

## 示例帧

连接（首帧）：

```json
{
  "type": "req",
  "id": "c1",
  "method": "connect",
  "params": {
    "minProtocol": 3,
    "maxProtocol": 3,
    "client": {
      "id": "openacosmi-cli",
      "displayName": "cli",
      "version": "1.0.0",
      "platform": "macos 15.1",
      "mode": "cli",
      "instanceId": "A1B2"
    }
  }
}
```

响应：

```json
{
  "type": "res",
  "id": "c1",
  "ok": true,
  "payload": {
    "type": "hello-ok",
    "protocol": 3,
    "server": { "version": "dev", "connId": "ws-1" },
    "features": { "methods": ["health"], "events": ["tick"] },
    "snapshot": {
      "presence": [],
      "health": {},
      "stateVersion": { "presence": 0, "health": 0 },
      "uptimeMs": 0
    }
  }
}
```

事件：

```json
{ "type": "event", "event": "tick", "payload": { "ts": 1730000000 }, "seq": 12 }
```

## 版本控制与兼容性

- 协议版本在 Go Gateway 中定义。
- 客户端发送 `minProtocol` + `maxProtocol`；服务端拒绝不匹配的连接。
- JSON field name 保持 camelCase 以兼容 Rust CLI（`#[serde(rename_all = "camelCase")]`）。

## 添加新方法（端到端步骤）

1. 在 `backend/internal/gateway/protocol/` 中定义 Go 结构体。
2. 在 `backend/internal/gateway/server_methods_*.go` 中添加处理器。
3. 在 `backend/internal/gateway/server.go` 中注册方法。
4. 如需 Rust CLI 支持，在 `cli-rust/crates/oa-types/` 中添加兼容类型。
5. 添加测试和文档。
