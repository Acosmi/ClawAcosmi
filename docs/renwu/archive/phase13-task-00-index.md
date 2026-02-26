> 📄 分块 00/08 | 索引 | 完整文件：phase13-task.md

# Phase 13 — 功能补缺实施 任务清单

> 范围：基于 S1-S6 生产级审计 + gap-analysis 差距分析（9 篇文档）
>
> 审计复核日期：2026-02-18
>
> **执行顺序**：D-W0 → A-W1 → A-W2 → A-W3a → A-W3b → C-W1 → B-W1 → B-W2 → B-W3 → D-W1 → D-W1b → D-W2 → F-W1 → F-W2 → G-W1 → G-W2
>
> **参考文档**：
>
> - 差距分析：`brain/0466828e-*/gap-analysis-part1~4f.md`
> - 最终执行计划：`brain/0466828e-*/final-execution-plan.md`
> - 实施计划原稿：`brain/4cb3ce79-*/implementation_plan.md.resolved`

---

## 窗口总览（17 窗口 → 19 会话）

| 序号 | 窗口 | 会话数 | 内容摘要 | 优先级 |
|------|------|--------|----------|--------|
| 1 | D-W0 | 1 | P12 剩余项（requestJSON/allowlist/proxy） | ✅ 完成 |
| 2 | A-W1 | 1 | 工具基础层 + agents/schema/ | ✅ 完成 |
| 3 | A-W2 | 1 | 文件/会话/媒体工具 | ✅ 完成 |
| 4 | A-W3a | 1 | 频道操作工具（Discord/Slack/Telegram/WA） | ✅ 完成 |
| 5 | A-W3b | 1 | Bash/PTY/补丁/子代理 ✅已完成 | ✅ 完成 |
| 6 | C-W1 | **2** | 沙箱（16文件）+ security 补全 | ✅ 完成 |
| 7 | B-W1 | 1 | 计费/用量追踪（8文件） | ✅ 完成 |
| 8 | B-W2 | 1 | 迁移/配对/远程 | ✅ 完成 |
| 9 | B-W3 | 1 | infra 补全（exec_approvals+heartbeat） | ✅ 完成 |
| 10 | D-W1 | **2** | Gateway 44 个 stub → 真实实现 | ✅ 完成 |
| 11 | D-W1b | 1 | Gateway 非stub方法补全 | ✅ 完成 |
| 12 | D-W2 | **2** | Auth+Skills三件套+Extensions | ✅ 完成 |
| 13 | F-W1 | 1 | CLI 命令注册 | ✅ 完成 |
| 14 | F-W2 | 1 | TUI bubbletea 渲染 | ✅ 完成 |
| 15 | G-W1 | 1 | 杂项+autoreply补全+WS排查 | ✅ 完成 |
| 16 | G-W2 | 1 | LINE channel SDK 完整实现 | ✅ 完成 |

> **合计**：17 窗口 → **19 个会话**（C-W1/D-W1/D-W2 各拆 2 会话）
>
> ⚠️ **注意**：A 组（A-W1~A-W3b）与 B 组（B-W1~B-W3）无依赖关系，可并行推进。

---

## 依赖关系图

```
D-W0 → A-W1 → A-W2 → A-W3a
                    → A-W3b → C-W1（sandbox 依赖 bash-tools）

B-W1 → B-W2 → B-W3          ← 与 A 组可并行

D-W1 → D-W1b
D-W2（需 A-W1 工具框架 + D-W1 skills stub）

F-W1 → F-W2（CLI 先于 TUI）

G-W1 → G-W2（杂项先于 LINE）
```

---

## 每会话启动协议

每个新会话窗口开始时需提供：

1. 对应 `gap-analysis-part4*.md` 中的任务清单
2. 相关 TS 源文件路径
3. 当前 Go 代码位置
4. `/refactor` 工作流引用
5. 前序窗口的完成状态

---

## 验证策略（每窗口完成后执行）

```bash
cd backend && go build ./... && go vet ./... && go test -race ./...
```

---
