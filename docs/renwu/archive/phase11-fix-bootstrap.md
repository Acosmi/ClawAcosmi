# Phase 11 修复 — 新窗口上下文 (Bootstrap)

> 用途：复制此文件内容到新窗口，快速恢复修复上下文
> 创建时间：2026-02-17

---

## 当前阶段

**Phase 11 修复** — 基于 6 个模块审计报告，按 P0→P1→P2 优先级分 Batch 修复

## 文档导航

| 文件 | 用途 |
|------|------|
| [phase11-fix-task.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-fix-task.md) | ✅ 任务 checklist（**先读这个**） |
| [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md) | 📌 延迟项汇总 (P11 段落) |
| [refactor-plan-full.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/refactor-plan-full.md) | 📋 全局路线图 |

### 审计报告 (按需参考)

| 模块 | 报告 |
|------|------|
| A. Gateway 方法 | [phase11-gateway-methods-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-gateway-methods-audit.md) |
| B. Session 管理 | [phase11-session-mgmt-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-session-mgmt-audit.md) |
| C. AutoReply 管线 | [phase11-autoreply-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-autoreply-audit.md) |
| D. Agent Runner | [phase11-agent-runner-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-agent-runner-audit.md) |
| E. WS 协议 | [phase11-ws-protocol-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-ws-protocol-audit.md) |
| F. Config/Scope | [phase11-config-scope-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-config-scope-audit.md) |

---

## 新窗口启动模板

```
请读取以下文件恢复上下文：

1. 技能工作流: .agent/workflows/refactor.md
2. 编码规范: skills/acosmi-refactor/references/coding-standards.md（跳过 Rust/FFI 章节）
3. 任务清单: docs/renwu/phase11-fix-task.md
4. 延迟项: docs/renwu/deferred-items.md (搜索 "P11-" 段落，确认是否有新增项需同步)
5. 文档模板: skills/acosmi-refactor/references/doc-template.md

当前执行 Batch [X]，请从 phase11-fix-task.md 中找到对应的 checklist 开始修复。

⚠️ 已知问题: WS 频繁断连 (~20s close 1006)，详见 refactor-health-audit-task.md 已知问题 #6。
```

---

## 项目状态快照

### 编译状态

```bash
cd backend && go build ./... && go vet ./...  # ✅ 通过
cd backend && go test -race ./...              # ✅ 通过
```

### Go 模块核心目录 (审计时快照，修复过程中可能变化)

| 目录 | 文件数 | 职责 |
|------|--------|------|
| `internal/gateway/` | ~30 | HTTP/WS 网关 + 方法处理器 |
| `internal/autoreply/` | ~31 | 自动回复引擎根 |
| `internal/autoreply/reply/` | ~34 | 回复管线子包 |
| `internal/agents/runner/` | ~11 | Agent 引擎 runner |
| `internal/agents/models/` | ~4 | 模型选择/回退 |
| `internal/agents/prompt/` | ~2 | 系统提示词 |
| `internal/agents/auth/` | ~2 | 认证 Profile |
| `internal/config/` | ~22 | 配置加载/校验 |
| `internal/agents/scope/` | ~4 | Agent Scope |
| `internal/sessions/` | ~6 | Session 工具函数 |

---

## Batch 优先级概览

| Batch | 优先级 | 项数 | 核心内容 | 状态 |
|-------|--------|------|----------|------|
| A | P0 | 3 | `agent` 处理器 + SessionStore 持久化 + MaxPayload 修正 | ✅ |
| B | P0 | 5 | AutoReply dispatch/session/model + Agent 工具/截断 | ✅ |
| C | P1 | 4 | Session 键归一化 + 合并存储 + patch 字段 + 类型去重 | ✅ |
| D | P1 | 10 | Gateway usage/CRUD/send/agent.wait + Agent prompt/subscribe/images/google + activeRuns | ✅ |
| E | P1 | 7 | AutoReply queue/commands/streaming + WS 安全 3 项 | ✅ |
| F | P2 | 5+4 | 体验优化（sessions.list + AutoReply UX + WS + Config + Gateway）+ 4 EXTRA 审计修复 | ✅ |
| G | P3 | ~25 | 延迟 stub + 模块 B/D P2/P3 延迟 + WS 断连排查 | ⬜ |

> [!IMPORTANT]
> **执行顺序约束**：
>
> - Batch A 必须先完成（B2 依赖 A2 的 SessionStore 持久化）
> - Batch B 可在 A 完成后并行推进
> - Batch C-E 可在 B 完成后按模块分窗口并行
> - Batch F-G 无硬依赖，可随时穿插

---

## 关键 TS 源文件位置

| 功能 | TS 路径 | 行数 |
|------|---------|------|
| `agent` 处理器 | `src/gateway/server-methods/agent.ts` | 388 |
| Session Store | `src/config/sessions/store.ts` | 495 |
| AutoReply dispatch | `src/auto-reply/dispatch.ts` | 77 |
| model-selection | `src/auto-reply/reply/model-selection.ts` | 584 |
| queue/* followup | `src/auto-reply/reply/queue/` (8 文件) | 679 |
| pi-embedded-subscribe | `src/agents/pi-embedded-subscribe.ts` | 618 |
| system-prompt | `src/agents/system-prompt.ts` | 648 |
| bash-tools.exec | `src/agents/bash-tools.exec.ts` | 1630 |
| tool-result-truncation | `src/agents/tool-result-truncation.ts` | 328 |
| images.ts | `src/agents/pi-embedded-runner/run/images.ts` | 447 |
| google.ts | `src/agents/pi-embedded-runner/run/google.ts` | 393 |
| usage.ts | `src/gateway/server-methods/usage.ts` | 822 |
| agents.ts CRUD | `src/gateway/server-methods/agents.ts` | 320 |
