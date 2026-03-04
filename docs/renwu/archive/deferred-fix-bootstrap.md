# 延迟待办修复 — 新窗口 Bootstrap

> 最后更新：2026-02-16（Batch DF-C 完成）
> 任务文档：[deferred-fix-task.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-fix-task.md)

---

## 新窗口启动模板

复制以下内容到新窗口即可快速恢复上下文：

```
@/refactor 执行延迟待办修复。

任务文档：docs/renwu/deferred-fix-task.md
审计来源：docs/renwu/deferred-items.md
全局路线图：docs/renwu/refactor-plan-full.md

当前进度：请读取 deferred-fix-task.md 确认已完成和待办项。

本批次目标：[在此指定 Batch DF-A / DF-B / DF-C]
```

---

## 一、背景

2026-02-16 对 `deferred-items.md`（823 行）进行了源码级逐项审计，确认：

- ~75 项延迟待办中 66 项已清除 ✅
- 9 项未清除，编排为 4 批次（DF-A → DF-D）
- DF-A/B/C 已全部完成 ✅，DF-D 保留为低优先级

## 二、批次概览

| 批次 | 内容 | 优先级 | 预估文件数 | 预估行数 | 状态 |
|------|------|--------|-----------|----------|------|
| **DF-A** | Session 管理模块 | P1 阻塞生产 | 3 新 + 3 测试 | ~500L | ✅ |
| **DF-B** | chunkMarkdownIR 接入 + loadWebMedia 统一 | P2 功能完善 | 2 新 + 6 改 | ~200L | ✅ |
| **DF-C** | files.uploadV2 + Runner 集成测试 | P3 优化 | 1 改 + 1 新 | ~200L | ✅ |
| **DF-D** | Security Audit + Rust 候选 | P4 延迟 | 不实施 | — | ⬜ |

## 三、各批次详细指引

### Batch DF-A：Session 管理模块

**目标**：创建 `internal/agents/session/` 包，实现完整 session 文件管理。

**前置依赖**：

- `internal/session/types.go` — `SessionEntry` 类型定义（已存在）
- `internal/gateway/transcript.go` — JSONL 读写（已存在，可参考）
- `internal/agents/runner/` — AttemptRunner 需要 session 管理（已存在）

**TS 参考文件**：

- `src/config/sessions/` — 会话 key 解析、存储路径、恢复策略（已在 `internal/sessions/` 移植，注意区分）
- `src/gateway/session-utils.ts` — 历史管理、Token 截断
- `src/agents/system-prompt.ts` — 系统提示词组装

**关键注意事项**：

1. `internal/sessions/`（已存在）负责会话 key 解析和路径，`internal/agents/session/`（新建）负责 session 文件的实际 I/O
2. 文件锁使用 `sync.Mutex`（进程内）+ 可选 `flock`（跨进程），先只做进程内
3. Token 截断需要对接 `internal/agents/models/` 的 context window 信息
4. 与 `internal/gateway/transcript.go` 有功能重叠，需决定是否合并或保持分层

**验证**：

```bash
cd backend && go build ./... && go vet ./...
cd backend && go test -race ./internal/agents/session/...
```

---

### Batch DF-B：调用端接入

**DF-B1: chunkMarkdownIR 接入**

目标文件：

- `internal/channels/slack/format.go` L93 — `TODO(phase7): 接入完整的 chunkMarkdownIR 实现。`
- `internal/channels/telegram/format.go` L116 — `简化实现：单块输出（完整分块依赖 Phase 7 markdown/ir.go chunkMarkdownIR）。`

核心依赖（已就绪）：

- `pkg/markdown/ir.go` `ChunkMarkdownIR()` — 完整实现（L599-629）
- `pkg/markdown/render.go` `MarkdownToSlackMrkdwnChunks()` — Slack mrkdwn 渲染

**DF-B2: loadWebMedia 统一**

目标：将 `whatsapp/media.go` 的 `LoadWebMedia` 提取到 `pkg/media/web_media.go`。

当前状态：

- `whatsapp/media.go:28` — `func LoadWebMedia(source string) (*WebMedia, error)`
- `discord/send_media.go:29` — 独立实现
- `slack/send.go:23-24` — `TODO(phase7): 接入完整的 loadWebMedia`
- `telegram/send.go:477` — `TODO: Phase 6 — 媒体发送（需要 loadWebMedia 桩）`

注意：需要统一 `WebMedia` 类型到 `pkg/media/` 下。

**验证**：

```bash
cd backend && go test -race ./internal/channels/... ./pkg/media/...
```

---

### Batch DF-C：辅助补全 ✅

**DF-C1: files.uploadV2** — ✅ 完成

- `internal/channels/slack/client.go` 升级为 3 步 API：`getUploadURLExternal` → POST → `completeUploadExternal`
- `client_upload_test.go` 6 tests PASS

**DF-C2: Runner 集成测试** — ✅ 完成

- `internal/agents/runner/integration_test.go` 5 tests PASS
- llmclient (mock SSE) + tool_executor 端到端验证

---

## 四、编码规范提醒

1. 所有表述/文档使用中文，代码标识符使用英文
2. 接口在使用方定义（DI 模式）
3. 错误处理：`fmt.Errorf("xxx: %w", err)` 不可 panic
4. 详见 `skills/acosmi-refactor/references/coding-standards.md`

## 五、完成后文档更新

每个 Batch 完成后必须更新：

1. `deferred-items.md` — 对应条目标记 ✅
2. `deferred-fix-task.md` — 勾选完成项
3. `phase10-task.md` — 更新 10.4 节
4. `refactor-plan-full.md` — 如适用
