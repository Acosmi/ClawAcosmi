# Phase 7 Bootstrap 上下文

> 新窗口启动时读取此文件，快速恢复 Phase 7 工作上下文。

## 当前阶段

**Phase 7：辅助模块** — Batch A+B+C+D 全部完成 ✅（审计 A-）

## 前置完成状态

- Phase 1-6 全部完成 ✅
- Phase 7 Batch A 完成 ✅：markdown(5文件), linkparse(3文件), security(4文件)
- Phase 7 Batch B 完成 ✅：tts(8文件, 1606L), media(11文件, 1858L), understanding(15文件, 986L)
- Phase 7 Batch C 完成 ✅：memory(15文件), browser(10文件)
- Phase 7 Batch D 完成 ✅（窗口 1-3）：autoreply(38文件, 4929L) — D1-D3 完整 + D4-D6 骨架+审计

## 必读文档

| 文件 | 用途 |
|------|------|
| [phase7-task.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase7-task.md) | 当前任务 checklist |
| [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md) | 延迟待办（含 Phase 7A-D 延迟项） |
| [phase4-9-deep-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase4-9-deep-audit.md) | Phase 7 审计数据（§三 7.1-7.7） |
| [phase7-d456-hidden-dep-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase7-d456-hidden-dep-audit.md) | Batch D D4-D6 隐藏依赖审计（A-） |
| [refactor-plan-full.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/refactor-plan-full.md) | 全局路线图 |

## 批次执行顺序

| 批次 | 子任务 | TS 行数 | Go 产出 | 状态 |
|------|--------|---------|---------|------|
| A | 7.7 Markdown+Link, 7.3 Security | ~5,757 | 12 文件 | ✅ |
| B | 7.6 TTS, 7.5 Media | ~6,973 | 34 文件 | ✅ |
| C | 7.2 Memory, 7.4 Browser | ~17,479 | 25 文件 | ✅ |
| D | 7.1 AutoReply | ~22,028 | 38 文件 4,929L | ✅ 审计 A- |

## Phase 8 延迟项概览

以下子系统因依赖未移植模块，延迟到 Phase 8：

| 延迟项 | TS 行数 | 阻塞依赖 |
|--------|---------|----------|
| status.ts | ~430 | config/agents/channels/tts/memory |
| skill-commands.ts | ~300 | agents/skills |
| reply/agent-runner-* (7文件) | ~2,000 | session/agents/config |
| reply/commands-* (14文件) | ~3,500 | 各命令执行器 |
| reply/directive-handling-* (10文件) | ~2,500 | model/thinking/tts 指令 |
| dispatch_from_config 完善 | 骨架→完整 | agent-runner + session |
| sanitizeUserFacingText | ~50 | agents 模块 |
| abort.go ABORT_MEMORY | ~200 | session/agent-runner/subagent |

---

## 新窗口启动模板

在新窗口中粘贴以下内容（用于 Phase 8 或后续任务）：

```
请阅读以下文件获取项目上下文：
1. `skills/acosmi-refactor/SKILL.md`
2. `skills/acosmi-refactor/references/coding-standards.md`
3. `docs/renwu/phase7-bootstrap.md`
4. `docs/renwu/phase7-task.md`
5. `docs/renwu/deferred-items.md` (Phase 7 Batch D + Phase 8 部分)
6. `docs/gouji/autoreply.md`

Phase 7 已全部完成（审计 A-）。请确认下一步任务。

@/refactor
```
