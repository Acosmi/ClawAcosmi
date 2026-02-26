# Phase 9 延迟项清理 — Bootstrap 上下文

> 创建时间：2026-02-15
> 前置：Phase 1-8 ✅、Pre-Phase 9 ✅

---

## 新窗口启动模板

在新窗口中粘贴以下内容即可恢复全部上下文：

```
@/refactor 执行 Phase 9 延迟项清理。

请先读取以下文件获取上下文：
1. docs/renwu/phase9-bootstrap.md（本文件）
2. docs/renwu/phase9-task.md — 分批任务清单
3. docs/renwu/deferred-items.md — 延迟项详细描述
4. docs/renwu/refactor-plan-full.md — 宏观路线图（Phase 9 章节）

当前要执行的批次是：Batch [D]，具体窗口是：[D2: ACP 辅助]。
```

---

## 当前阶段概述

**Phase 9 目标**：消化 `deferred-items.md` 中 ~60 项未解决延迟待办，为 Phase 10 集成测试扫清障碍。

**批次划分**：

| 批次 | 内容 | 预估项数 | 建议窗口 |
|------|------|----------|----------|
| Batch A | Gateway 集成 | ~35 项 | 6 窗口（按频道） |
| Batch B | Config/Agent 补全 | ~10 项 | 2-3 窗口 |
| Batch C | Agent 引擎缺口 | ~8 项 | 2-3 窗口 |
| Batch D | 辅助/优化 | ~15 项 | 3-4 窗口 |

**执行顺序建议**：B → C → A → D（先补全基础设施，再做 Gateway 连线，最后优化）

---

## 前置条件

- Phase 1-8 全部完成 ✅
- Pre-Phase 9（WA-WD）延迟项清理 ✅
- Go 编译 `go build ./...` 通过 ✅
- 所有已有测试 `go test -race ./...` 通过 ✅

## 关键文档

| 文档 | 用途 |
|------|------|
| [phase9-task.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase9-task.md) | 分批任务清单 |
| [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md) | 延迟项详细描述 + 代码位置 |
| [refactor-plan-full.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/refactor-plan-full.md) | 宏观路线图 |
| [phase4-9-deep-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase4-9-deep-audit.md) | 深度审计参考 |

## 工作流提醒

- 遵循 `/refactor` 六步法，每个 item 独立执行 Step 1-6
- 每完成一项：更新 `deferred-items.md` 标记 ✅ + 更新 `phase9-task.md` checklist
- 架构文档：完成模块后更新/创建 `docs/gouji/{模块名}.md`
- 编译验证：`cd backend && go build ./... && go vet ./... && go test -race ./internal/...`
