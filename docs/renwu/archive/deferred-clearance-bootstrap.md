# 延迟项清除 — 新窗口启动上下文

> 本文件供每个新窗口快速恢复上下文。

## 新窗口启动模板

> 在新窗口中粘贴以下内容即可启动工作：

```
@refactor

延迟项清除窗口 W{N}。请读取以下文件恢复上下文：

1. docs/renwu/deferred-clearance-task.md — 找到 Window {N} 的子任务清单
2. docs/renwu/deferred-items.md — 当前延迟项详情
3. skills/acosmi-refactor/references/coding-standards.md（跳过 Rust/FFI 章节）

执行 Window {N} 的所有子任务。遵循六步循环法。
```

---

## 项目状态快照

- **Phase 11 审计 + 修复** ✅ Batch A-G 全部完成
- **剩余延迟项**：24 项待清除 + 2 项推迟 Phase 12（Ollama/i18n）
- **编译状态**：`go build` ✅ | `go vet` ✅ | `go test -race` ✅

---

## 关键路径和目录映射

| 模块 | Go 路径 | TS 参考路径 |
|------|---------|-------------|
| Sessions 去重 | `internal/sessions/sessions.go` | `src/config/sessions/` |
| 模型选择 | `internal/agents/models/selection.go` | `src/agents/model-selection.ts` |
| AutoReply 队列 | `internal/autoreply/reply/queue_*.go` | `src/auto-reply/reply/queue/` |
| Agent Runner | `internal/agents/runner/` | `src/agents/pi-embedded-runner/` |
| Channel Dock | `internal/channels/dock.go` | `src/channels/dock.ts` |
| WS 安全 | `internal/gateway/device_auth.go` | `src/gateway/device-pairing.ts` |
| Config 校验 | `internal/config/validator.go` | `src/config/validation.ts` |
| Maintenance | `internal/gateway/maintenance.go` | `src/gateway/server-maintenance.ts` |
| WS 日志 | `internal/gateway/ws_log.go` | `src/gateway/ws-log.ts` |

---

## 窗口依赖关系

```
W1 → W2 → W3        （独立链）
W4 → W5              （Channel Dock 链，依赖 W4 前置骨架）
W6                   （独立）
W7                   （独立）
W8                   （独立，需 chatRunState 基础设施）
W9                   （独立，跨前后端）
```

W1-W3 和 W4-W5 可并行。W6-W9 各自独立。

---

## 验证命令

每个窗口完成后必须通过：

```bash
cd backend && go build ./... && go vet ./... && go test -race ./...
```

---

## 文档更新规范

每个窗口完成后：

1. ✅ Update `deferred-clearance-task.md` checklist
2. ✅ Update `deferred-items.md` 对应项状态 → `✅ 已完成`
3. ✅ 如完成整个 Phase → update `refactor-plan-full.md`
