---
summary: "Gateway WebSocket 协议：握手、帧格式、版本控制"
read_when:
  - 实现或更新 Gateway WS 客户端
  - 调试协议不匹配或连接失败
title: "Gateway 协议"
---

# Gateway 协议（WebSocket）

> [!IMPORTANT]
> **架构状态**：协议由 **Go Gateway**（`backend/internal/gateway/protocol.go`）定义，
> 帧解析在 `ws_server.go` 中实现。当前协议版本为 `ProtocolVersion = 3`。

Gateway WS 协议是 OpenAcosmi 的**统一控制面 + 节点传输**。
所有客户端（CLI、Web UI、macOS 应用、iOS/Android 节点、无头节点）
通过 WebSocket 连接，并在握手时声明 **角色** + **作用域**。

## 传输层

- WebSocket，文本帧 + JSON 载荷。
- 首帧**必须**是 `connect` 请求。

## 握手（connect）

Gateway → 客户端（连接前质询）：

```json
{
  "type": "event",
  "event": "connect.challenge",
  "payload": { "nonce": "…", "ts": 1737264000000 }
}
```

客户端 → Gateway：

```json
{
  "type": "req",
  "id": "…",
  "method": "connect",
  "params": {
    "minProtocol": 3,
    "maxProtocol": 3,
    "client": {
      "id": "cli",
      "version": "1.2.3",
      "platform": "macos",
      "mode": "operator"
    },
    "role": "operator",
    "scopes": ["operator.read", "operator.write"],
    "caps": [],
    "commands": [],
    "permissions": {},
    "auth": { "token": "…" },
    "locale": "zh-CN",
    "userAgent": "openacosmi-cli/1.2.3",
    "device": {
      "id": "device_fingerprint",
      "publicKey": "…",
      "signature": "…",
      "signedAt": 1737264000000,
      "nonce": "…"
    }
  }
}
```

Gateway → 客户端：

```json
{
  "type": "res",
  "id": "…",
  "ok": true,
  "payload": { "type": "hello-ok", "protocol": 3, "policy": { "tickIntervalMs": 15000 } }
}
```

签发设备 token 时，`hello-ok` 还包含：

```json
{
  "auth": {
    "deviceToken": "…",
    "role": "operator",
    "scopes": ["operator.read", "operator.write"]
  }
}
```

### 节点示例

```json
{
  "type": "req",
  "id": "…",
  "method": "connect",
  "params": {
    "minProtocol": 3,
    "maxProtocol": 3,
    "client": {
      "id": "ios-node",
      "version": "1.2.3",
      "platform": "ios",
      "mode": "node"
    },
    "role": "node",
    "scopes": [],
    "caps": ["camera", "canvas", "screen", "location", "voice"],
    "commands": ["camera.snap", "canvas.navigate", "screen.record", "location.get"],
    "permissions": { "camera.capture": true, "screen.record": false },
    "auth": { "token": "…" }
  }
}
```

## 帧格式

Go 结构体定义在 `protocol.go` 中：

- **请求（RequestFrame）**：`{type:"req", id, method, params}`
- **响应（ResponseFrame）**：`{type:"res", id, ok, payload|error}`
- **事件（EventFrame）**：`{type:"event", event, payload, seq?, stateVersion?}`

有副作用的方法需要**幂等键**（见 `idempotency.go`）。

## 角色 + 作用域

### 角色

- `operator` = 控制面客户端（CLI/UI/自动化）。
- `node` = 能力宿主（摄像头/屏幕/Canvas/system.run）。

### 作用域（operator）

常用作用域：

- `operator.read`
- `operator.write`
- `operator.admin`
- `operator.approvals`
- `operator.pairing`

### Caps/commands/permissions（node）

节点在连接时声明能力：

- `caps`：高阶能力类别。
- `commands`：可调用命令白名单。
- `permissions`：细粒度开关（如 `screen.record`、`camera.capture`）。

Gateway 将其视为**声明**并在服务端执行白名单校验。

## Presence

- `system-presence` 返回按设备标识为键的条目（`PresenceEntry` 结构体）。
- Presence 条目包含 `deviceId`、`roles` 和 `scopes`，UI 可对每设备显示一行。

### 节点辅助方法

- 节点可调用 `skills.bins` 获取当前技能可执行文件列表。

## 执行审批

- 当执行请求需要审批时，Gateway 广播 `exec.approval.requested`。
- Operator 客户端通过调用 `exec.approval.resolve` 解决（需 `operator.approvals` 作用域）。

## 版本控制

- `ProtocolVersion` 定义在 `backend/internal/gateway/protocol.go`（当前值：3）。
- 客户端发送 `minProtocol` + `maxProtocol`；服务端拒绝不匹配的版本。
- Go Gateway 使用 Go 结构体定义协议类型，不依赖外部代码生成。

## 认证

- 如果设置了 `OPENACOSMI_GATEWAY_TOKEN`（或 `--token`），`connect.params.auth.token` 必须匹配，否则关闭连接。
- 配对后，Gateway 签发**设备 token**，作用域限于连接角色 + 作用域。
  返回在 `hello-ok.auth.deviceToken` 中，客户端应持久化以供后续连接。
- 设备 token 可通过 `device.token.rotate` 和 `device.token.revoke` 轮换/撤销。

## 设备身份 + 配对

- 节点应包含稳定的设备标识（`device.id`），基于密钥对指纹。
- Gateway 按设备 + 角色签发 token。
- 新设备 ID 需配对审批，除非启用本地自动审批。
- **本地**连接包括 loopback 和 Gateway 宿主的 tailnet 地址。
- 所有 WS 客户端必须在 `connect` 时包含 `device` 身份。
- 非本地连接必须签名服务端提供的 `connect.challenge` nonce。

## TLS + 证书锁定

- WS 连接支持 TLS（`tls_runtime.go`、`tls_gateway.go`）。
- 客户端可选择锁定 Gateway 证书指纹。

## 错误码

Go Gateway 定义了 24 个错误码（`protocol.go` 中的 `ErrCode*` 常量）：

**原有错误码**（5 个）：`bad_request`、`unauthorized`、`not_found`、`internal_error`、`not_implemented`

**Go 新增错误码**（19 个）：`forbidden`、`conflict`、`payload_too_large`、`too_many_requests`、`service_unavailable`、`protocol_mismatch`、`session_not_found`、`agent_not_found`、`agent_busy`、`timeout`、`aborted` 等。

## 作用范围

此协议暴露**完整 Gateway API**（status、channels、models、chat、agent、sessions、nodes、approvals 等）。
所有类型定义在 `backend/internal/gateway/protocol.go` 和 `pkg/types/types_gateway.go` 中。
