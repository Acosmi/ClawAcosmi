# 模块 A: Gateway 方法 — 审计 Bootstrap

> 用于新窗口快速恢复上下文

---

## 新窗口启动模板

在新窗口中粘贴以下内容开始审计：

```
请执行 Gateway 方法模块的重构健康度审计。

## 上下文
1. 读取审计总表: `docs/renwu/refactor-health-audit-task.md`
2. 读取本 bootstrap: `docs/renwu/phase11-gateway-methods-bootstrap.md`
3. 读取 `/refactor` 技能工作流
4. 读取编码规范: `skills/acosmi-refactor/references/coding-standards.md`
5. 读取 `docs/renwu/deferred-items.md`

## 目标
对比 TS 原版 `src/gateway/server-methods/` 与 Go 移植 `backend/internal/gateway/server_methods_*.go`，
执行六步循环法审计，找出行为偏差和缺失功能。
```

---

## TS 源文件清单 (`src/gateway/server-methods/`)

| 文件 | 大小 | 对应 Go 文件 |
|------|------|-------------|
| `chat.ts` | 22KB | `server_methods_chat.go` (13KB) |
| `sessions.ts` | 15KB | `server_methods_sessions.go` (17KB) |
| `agents.ts` | 15KB | `server_methods_agents.go` (3KB) + `server_methods_agent.go` (2KB) |
| `agent.ts` | 18KB | `server_methods_agent.go` + `server_methods_agent_files.go` (9KB) |
| `config.ts` | 13KB | `server_methods_config.go` (9KB) |
| `usage.ts` | 27KB | `server_methods_usage.go` (3KB) ⚠️ 大幅缩减 |
| `send.ts` | 12KB | `server_methods_send.go` (3KB) |
| `channels.ts` | 10KB | `server_methods_channels.go` (6KB) |
| `nodes.ts` | 17KB | `server_methods_stubs.go` (stub) ⚠️ |
| `browser.ts` | 8KB | `server_methods_stubs.go` (stub) ⚠️ |
| `skills.ts` | 7KB | `server_methods_stubs.go` (stub) ⚠️ |
| `exec-approvals.ts` | 5KB + `exec-approval.ts` 7KB | `server_methods_exec_approvals.go` (7KB) |
| `devices.ts` | 5KB | `server_methods_stubs.go` (stub) ⚠️ |
| `system.ts` | 5KB | `server_methods_system.go` (6KB) |
| `logs.ts` | 5KB | `server_methods_logs.go` (5KB) |
| `cron.ts` | 7KB | `server_methods_stubs.go` (stub) ⚠️ |
| `tts.ts` | 5KB | `server_methods_stubs.go` (stub) ⚠️ |
| `health.ts` | 1KB | `server_methods_system.go` (内联) |
| `models.ts` | 1KB | `server_methods_models.go` (1KB) |
| `wizard.ts` | 4KB | `server_methods_stubs.go` (stub) ⚠️ |
| `web.ts` | 4KB | `server_methods_stubs.go` (stub) ⚠️ |
| `talk.ts` | 1KB | `server_methods_stubs.go` (stub) ⚠️ |
| `types.ts` | 4KB | `server_methods.go` (类型内联) |

## 关键审计点

1. **Stubs 覆盖率**: 多少 RPC 方法仍是 stub？哪些是必须实现的？
2. **chat.ts 一致性**: 对比 `broadcastChatFinal` 行为，`dispatchInboundMessage` 调用链
3. **sessions.ts 一致性**: `sessions.resolve` / `sessions.patch` 行为
4. **usage.ts 缩减**: 27KB → 3KB，大量功能缺失
5. **send.ts 外部消息**: 向 Slack/Telegram 等渠道发送消息的逻辑

## 已知问题

- `chat.send` 中 session 自动创建已修复（本轮调试）
- `message` vs `text` 字段名已修复
- Token 传递已修复
