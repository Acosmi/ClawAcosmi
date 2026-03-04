# Phase 13 — Bootstrap 启动上下文

> 最后更新：2026-02-18
>
> **Phase 13 目标**：基于 S1-S6 生产级审计 + gap-analysis 差距分析，补全 TS→Go 重构遗漏的全部功能模块
>
> **任务索引**：[phase13-task-00-index.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase13-task-00-index.md)

---

## 整体进度快照

| 模块组 | 窗口 | 状态 |
|--------|------|------|
| D-W0 | P12 剩余项 | ⬜ 待执行 |
| A-W1~A-W3b | Agent 工具系统 | ⬜ 待执行 |
| C-W1 | 沙箱+安全 | ⬜ 待执行 |
| B-W1~B-W3 | infra 补缺 | ⬜ 待执行 |
| D-W1~D-W2 | Gateway+Auth+Skills | ⬜ 待执行 |
| F-W1~F-W2 | CLI+TUI | ⬜ 待执行 |
| G-W1~G-W2 | 杂项+LINE | ⬜ 待执行 |

---

## 关键背景数据

| Phase | 模块 | 完成度 |
|-------|------|--------|
| 1 | config/types | **85%** |
| 2 | infra | **30%** 🔴 |
| 3 | gateway | **70%** |
| 4 | agents | **35%** 🔴 |
| 5 | channels | **75%** |
| 6 | cli/plugins/hooks | **65%** |
| 7 | aux modules | **65%** |
| 8-12 | autoreply/延迟项 | **~90%** |

**整体完成度：~62%**（Phase 1-7 按行数加权）

---

## 窗口 → 分块文件映射表

| 窗口 | 任务分块文件 | TS 源目录 | Go 目标目录 |
|------|-------------|-----------|-------------|
| D-W0 | `phase13-task-01-DW0.md` | `src/node-host/` | `internal/nodehost/` |
| A-W1 | `phase13-task-02-AW1-AW2.md` | `src/agents/tools/` + `src/agents/schema/` | `internal/agents/tools/` + `internal/agents/schema/` |
| A-W2 | `phase13-task-02-AW1-AW2.md` | `src/agents/tools/` | `internal/agents/tools/` |
| A-W3a | `phase13-task-03-AW3.md` | `src/agents/tools/*-actions.ts` | `internal/agents/tools/` |
| A-W3b | `phase13-task-03-AW3.md` | `src/agents/bash-tools.*` | `internal/agents/bash/` |
| C-W1 | `phase13-task-04-CW1-BW1.md` | `src/agents/sandbox/` + `src/security/` | `internal/agents/sandbox/` + `internal/security/` |
| B-W1 | `phase13-task-04-CW1-BW1.md` | `src/infra/session-cost-usage.ts` + `src/infra/provider-usage.*.ts` | `internal/infra/cost/` |
| B-W2 | `phase13-task-05-BW2-BW3.md` | `src/infra/state-migrations.ts` + `src/infra/exec-approval-forwarder.ts` | `internal/infra/` |
| B-W3 | `phase13-task-05-BW2-BW3.md` | `src/infra/exec-approvals.ts` + `src/infra/heartbeat-runner.ts` | `internal/infra/` |
| D-W1 | `phase13-task-06-DW1.md` | `src/gateway/server-methods/` | `internal/gateway/` |
| D-W1b | `phase13-task-07-DW1b-DW2.md` | `src/gateway/server-methods/` + `src/gateway/protocol/` | `internal/gateway/` |
| D-W2 | `phase13-task-07-DW1b-DW2.md` | `src/agents/auth-profiles/` + `src/agents/skills/` + `src/agents/pi-extensions/` | `internal/agents/auth/` + `internal/agents/skills/` + `internal/agents/extensions/` |
| F-W1 | `phase13-task-08-FG.md` | `src/commands/` + `src/cli/` | `cmd/openacosmi/` + `internal/cli/` |
| F-W2 | `phase13-task-08-FG.md` | `src/tui/` | `internal/tui/` |
| G-W1 | `phase13-task-08-FG.md` | `src/infra/` + `src/autoreply/` | `internal/infra/` + `internal/autoreply/` |
| G-W2 | `phase13-task-08-FG.md` | `src/line/` | `internal/channels/line/` |

---

## ⚠️ 执行前必读风险点

### 风险 1：B-W2 开始前必须侦察 forwarder 现状

```bash
find backend/internal/infra -name "*approval*" -o -name "*forwarder*"
```

### 风险 2：D-W1b 开始前必须侦察 protocol/ 实际缺口

```bash
find src/gateway/protocol -name '*.ts' | xargs wc -l | sort -n
grep -r "type.*Protocol\|type.*Message" backend/internal/gateway/ --include="*.go" -l
```

### 风险 3：D-W0 requestJSON 注入点可能超估算

### 风险 4：A-W3b Bash/PTY/Docker 是高风险模块，不与其他窗口并行

---

## 重要文档索引

| 文档 | 路径 |
|------|------|
| 任务索引 | `docs/renwu/phase13-task-00-index.md` |
| 任务分块 01~08 | `docs/renwu/phase13-task-01-DW0.md` ~ `phase13-task-08-FG.md` |
| 完整任务（归档） | `docs/renwu/phase13-task.md` |
| 审计报告 S1-S6 | `docs/renwu/production-audit-s1~s6.md` |
| 延迟待办 | `docs/renwu/deferred-items.md` |
| 全局路线图 | `docs/renwu/refactor-plan-full.md` |
