# 审计修复 Bootstrap 上下文

> 新窗口上下文 | 前置: 2026-02-16 全局深度审计已完成

## 当前状态摘要

- 已完成：全项目深度审计 (89 files)，报告已生成
- 已完成：Batch A (`accessibility.rs` 内存安全修复) + Batch B (Go 侧快速修复)
- 已延迟：Batch C (C1-C3 P2 可选修复) → `deferred-items.md`
- 已延迟：文件拆分 (20 files > 300L) — 独立重构任务

## 本阶段目标

修复审计发现的 3 个 P1 + 选修 P2 问题，按 Batch A → B → C 执行。

## 关键文件索引

| 文件 | 用途 |
|------|------|
| `docs/renwu/task-20260216-audit-fixes.md` | **任务跟踪** — 含状态标记和完成后文档更新清单 |
| `docs/renwu/audit-20260216-global-deep.md` | 审计报告（发现详情） |
| `rust-core/src/accessibility.rs` | P1 BUG-D2-01/02 所在文件 (661L) |
| `go-sensory/internal/vlm/router.go` | P1 BUG-D2-03 — body defer |
| `go-sensory/internal/api/dashboard.go` | P2 STYLE-D2-02 — panic |
| `go-sensory/internal/vlm/health.go` | P2 SEC-D2-01 — Gemini key in URL |

## 遵循的规范

- 工作流: `.agent/workflows/1-refactor.md`
- 编码规范: `.agent/skills/acosmi-refactor/references/coding-standards.md`
- FFI 规范: `.agent/skills/acosmi-refactor/references/ffi-conventions.md`

## 新窗口启动指令

```
/1-refactor
请读取 docs/renwu/bootstrap-audit-fixes.md 和 docs/renwu/task-20260216-audit-fixes.md，
按 /refactor 工作流执行审计修复任务（Batch A → B → C）。
每完成一个 Batch 后更新 task 文档中的状态标记。
全部完成后更新"完成后需更新的文档"清单中的相关文档。
```
