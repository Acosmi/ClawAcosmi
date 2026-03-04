# 延迟待办修复 Phase 2 — 新窗口 Bootstrap

> 最后更新：2026-02-16
> 任务文档：[deferred-fix2-task.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-fix2-task.md)

---

## 新窗口启动模板

复制以下内容到新窗口即可快速恢复上下文：

```
@/refactor 执行延迟待办修复 Phase 2。

任务文档：docs/renwu/deferred-fix2-task.md
审计来源：docs/renwu/deferred-items.md（末尾 3 章节：缺口 A/B/E）
全局路线图：docs/renwu/refactor-plan-full.md

当前进度：请读取 deferred-fix2-task.md 确认已完成和待办项。

本批次目标：[在此指定 Batch DF2-A / DF2-B / DF2-C]
```

---

## 一、背景

2026-02-16 对 `deferred-items.md` 进行了扩大审计（全量 TODO/stub/skeleton 源码扫描），发现 3 类未在原文档中追踪的缺口：

| 缺口 | 来源 | 项数 | 优先级 |
|------|------|------|--------|
| **A: Telegram Phase 6** | `internal/channels/telegram/` | 19 处 TODO | P1 高 |
| **B: Bridge Actions** | `internal/channels/bridge/` | 12 处桩 | P2 中 |
| **E: 内部模块骨架** | memory/cron/outbound 等 | 7 处骨架 | P2-P3 中 |

这些缺口已追加到 `deferred-items.md` 末尾并编排为 3 批次（DF2-A → DF2-C）。

---

## 二、批次概览

| 批次 | 内容 | 优先级 | 预估文件数 | 状态 |
|------|------|--------|-----------|------|
| **DF2-A** | Telegram Phase 6 集成 | P1 高 | ~10 改 | ⬜ |
| **DF2-B** | Bridge Actions 实现 | P2 中 | 3 改 | ⬜ |
| **DF2-C** | 内部模块残留骨架 | P2-P3 | ~7 改 | ⬜ |

---

## 三、各批次关键指引

### Batch DF2-A：Telegram Phase 6 集成

**目标**：将 Telegram 从骨架状态升级为完整集成（对齐 iMessage/Signal/Slack/Discord）。

**前置已就绪**：

- `internal/agents/session/` — session 管理模块 ✅
- `internal/autoreply/reply/` — auto-reply 引擎 ✅
- `internal/routing/session_key.go` — session key 路由 ✅
- `pkg/media/web_media.go` — 统一媒体加载 ✅
- `pkg/markdown/ir.go` — Markdown IR 分块 ✅

**参考实现**（已完成的频道，可参考模式）：

- **iMessage**：`internal/channels/imessage/monitor_inbound.go` — 640L 完整管线
- **Slack**：`internal/channels/slack/monitor_message_dispatch.go` — agent 路由 + 分发
- **Discord**：`internal/channels/discord/monitor_message_dispatch.go` — 消息管线

**TS 参考**：

- `src/channels/telegram/bot.ts` — bot 入口
- `src/channels/telegram/bot-message.ts` — 消息处理
- `src/channels/telegram/bot-response.ts` — 回复管理

**验证**：

```bash
cd backend && go build ./... && go vet ./...
cd backend && go test -race ./internal/channels/telegram/...
```

---

### Batch DF2-B：Bridge Actions

**目标**：实现 3 个频道的 messaging tool 动作执行。

**前置已就绪**：

- `internal/channels/bridge/types.go` — ToolResult 类型 ✅
- 各频道的 `client.go` / `send.go` API ✅

**关键注意事项**：

1. Bridge Actions 是 messaging tool 在各频道的动作分发层
2. 需调用各频道的 send/reaction/pin 等已有 API
3. Telegram 的 bridge actions 依赖 DF2-A 的媒体发送完成

**验证**：

```bash
cd backend && go test -race ./internal/channels/bridge/...
```

---

### Batch DF2-C：内部模块残留骨架

**目标**：填充核心功能的 skeleton/stub。

**依赖关系**：

- INT-1 (Memory Search) 依赖 embeddings + sqlite_vec（均已就绪）
- INT-2 (Cron Runner) 依赖 `autoreply/reply/AgentExecutor`（已就绪）
- INT-3 (Outbound Deliver) 依赖 `pkg/markdown/ir.go`（已就绪）
- INT-4 (FollowupRunner) 无外部依赖
- INT-5~7 低优先级，可延后

**验证**：

```bash
cd backend && go build ./... && go vet ./...
cd backend && go test -race ./internal/memory/... ./internal/cron/... ./internal/outbound/...
```

---

## 四、编码规范提醒

1. 所有表述/文档使用中文，代码标识符使用英文
2. 接口在使用方定义（DI 模式）
3. 错误处理：`fmt.Errorf("xxx: %w", err)` 不可 panic
4. 详见 `skills/acosmi-refactor/references/coding-standards.md`

## 五、完成后文档更新

每个 Batch 完成后必须更新：

1. `deferred-items.md` — 对应缺口章节标记 ✅
2. `deferred-fix2-task.md` — 勾选完成项
3. `refactor-plan-full.md` — 如适用
