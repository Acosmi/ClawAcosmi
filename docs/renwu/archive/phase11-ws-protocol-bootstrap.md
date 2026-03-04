# 模块 E: WS 协议 — 审计 Bootstrap

> 用于新窗口快速恢复上下文

---

## 新窗口启动模板

```
请执行 WS 协议模块的重构健康度审计。

## 上下文
1. 读取审计总表: `docs/renwu/refactor-health-audit-task.md`
2. 读取本 bootstrap: `docs/renwu/phase11-ws-protocol-bootstrap.md`
3. 读取 `/refactor` 技能工作流
4. 读取编码规范: `skills/acosmi-refactor/references/coding-standards.md`
5. 读取 `docs/renwu/deferred-items.md`
6. 控制输出量：预防上下文过载引发崩溃，需要大量输出时请逐步分段输出。
7. 任务完成后：请按要求更新 `refactor-plan-full.md` 和本模块的审计报告。

## 目标
对比 TS 原版 WS 服务端 (`src/gateway/server/ws-*.ts`) 与 Go 移植 (`backend/internal/gateway/ws*.go`)。

> **注意**: 具体审计步骤请严格参考 `docs/renwu/refactor-health-audit-task.md` 模块 E 章节。此文档仅提供上下文和文件索引。
```

---

## TS 源文件

| 文件 | 大小 | 职责 |
|------|------|------|
| `src/gateway/server/ws-connection.ts` | ~15KB | WS 连接管理、帧处理 |
| `src/gateway/server/ws-types.ts` | ~3KB | WS 帧类型定义 |
| `src/gateway/ws-log.ts` | ~3KB | WS 日志记录 |
| `src/gateway/ws-logging.ts` | ~3KB | WS 日志格式 |

## Go 对应文件

| 文件 | 大小 | 对应 TS |
|------|------|---------|
| `ws_server.go` | 9KB | `ws-connection.ts` (核心服务端) |
| `ws.go` | 6KB | WS 客户端 (outbound) |
| `ws_test.go` | 3KB | WS 测试 |
| `protocol.go` | 10KB | 帧类型 + 协议常量 |

## 关键审计点

1. **帧格式一致性**: 检查 `connect` / `hello-ok` / `request` / `response` / `event` 帧格式
2. **心跳机制**:
   - Go: 服务端 30s Ping + 90s ReadDeadline + PongHandler
   - TS: 检查原版是否相同参数
3. **重连策略**: 前端 `GatewayBrowserClient` 的重连参数是否与后端匹配
4. **错误码**: 检查 `4008`, `1006`, `1012` 等自定义关闭码的一致性
5. **连接限制**: TS 原版是否有最大连接数限制？Go 端是否实现？
6. **Buffered amount 跟踪**: Go 用 `atomic.Int64` 跟踪缓冲量，是否与 TS 行为一致？

## 已知问题

- WS 频繁断连 (~20s 间隔)，`close 1006 (abnormal closure): unexpected EOF`
- 可能是前端 GatewayBrowserClient 的重连逻辑导致
- 需与前端 `ui/src/ui/gateway.ts` 中的 `GatewayBrowserClient` 配合排查
