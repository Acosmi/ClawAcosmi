# Signal SDK 架构文档

> 最后更新：2026-02-26 | 代码级审计确认 | 14 源文件, 7 测试文件, 62 测试, ~3,153 行
> TS 源：`src/signal/` (14 文件)
> Go 文件：14 个
> 审计报告：[phase5d-signal-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase5d-signal-audit.md)

## 模块概览

Signal SDK 通过 signal-cli 的 JSON-RPC HTTP 接口与 Signal 通信，提供多账户管理、消息发送/接收、反应、SSE 事件流和守护进程管理。

## 文件清单

| Go 文件 | TS 源 | 职责 |
| ------- | ----- | ---- |
| `accounts.go` | `accounts.ts` | 多账户解析/合并配置/列出启用账户 |
| `identity.go` | `identity.ts` | 发送者身份解析（phone/uuid）、allowlist 过滤 |
| `reaction_level.go` | `reaction-level.ts` | 反应级别策略（off/ack/minimal/extensive） |
| `client.go` | `client.ts` | JSON-RPC 2.0 客户端 + SSE 事件流 + health check |
| `format.go` | `format.ts` | Markdown → Signal 文本样式转换 |
| `send.go` | `send.ts` | 消息发送 + 打字状态 + 已读回执 |
| `send_reactions.go` | `send-reactions.ts` | 反应发送/删除 |
| `daemon.go` | `daemon.ts` | signal-cli 守护进程生命周期管理 |
| `probe.go` | `probe.ts` | 健康探测 + 版本检测 |
| `sse_reconnect.go` | `sse-reconnect.ts` | SSE 指数退避重连循环 |
| `monitor.go` | `monitor.ts` | 监控入口 + 守护进程启动 + 事件分发 |
| `monitor_types.go` | `monitor/event-handler.types.ts` | 事件处理类型定义 |
| `event_handler.go` | `monitor/event-handler.ts` | 入站消息全量处理 |
| `index.go` | `index.ts` | 包导出 |

## 关键设计

### JSON-RPC 协议

- 端点：`POST /api/v1/rpc`（JSON-RPC 2.0）
- Health：`GET /api/v1/check`
- SSE：`GET /api/v1/events?account=xxx`
- text-style 格式：字符串数组 `"start:length:STYLE"`，key 为 `text-style`（kebab-case）

### 发送目标解析

- `recipient: ["+1234"]` — 电话号码/UUID
- `username: ["alice"]` — 用户名
- `groupId: "xxxx"` — 群组

### 隐藏依赖审计修复

| 项 | 修复内容 |
| -- | ------- |
| H1 | health check 端点 `/api/v1/about` → `/api/v1/check` |
| H2 | text-style 序列化：JSON 对象 → 字符串 `"0:5:BOLD"` + key `text-style` |
| M1 | username target 映射：`recipient` → `username: []` 数组 |
| M2 | HTTP 201 Created 视为成功 |
| M3 | RPC 超时可配置（默认 10s 匹配 TS） |

## 待实现（Phase 6/7）

- **5 处 TODO 桩函数**：消息分发管线、事件队列、配对注册、媒体下载、已读回执
- **format.go IR 管线**：Phase 7 共享 Markdown IR 包（链接处理、样式合并/钳位、表格模式）
- **normalizeAccountId**：待 Phase 2 routing/session-key 模块完成
