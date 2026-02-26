---
description: 新对话上下文初始化工作流。在开始新一轮改造对话时使用，自动加载上一阶段进度、延迟项和待办清单，生成新的 bootstrap 文档作为会话起始上下文。
---

# /bootstrap — 上下文初始化工作流

// turbo-all

## 步骤 0：加载技能

1. 读取技能入口：`.agent/skills/acosmi-refactor/SKILL.md`
2. 读取文档模板：`.agent/skills/acosmi-refactor/references/doc-template.md`（bootstrap 模板）
3. 读取重构排序：`.agent/skills/acosmi-refactor/references/refactor-order.md`

## 步骤 1：收集现有进度

1. 查找最新的 bootstrap 文档：`docs/renwu/bootstrap-*.md`
2. 读取延迟待办：`docs/renwu/deferred-items.md`（如存在）
3. 读取近期审计报告：`docs/renwu/audit-*.md`
4. 检查架构方案中各 Phase 的完成状态：`docs/jiagou/改造Go+Rust 混合架构.md`

## 步骤 2：确定当前阶段

1. 汇总已完成的 Phase/Batch 列表
2. 确定下一个待执行的 Phase/Batch
3. 向用户确认即将开始的工作范围

## 步骤 3：生成 Bootstrap 文档

1. 按 bootstrap 模板生成新文档：`docs/renwu/bootstrap-<phase>.md`
2. 包含：
   - 当前状态摘要（已完成 / 进行中 / 已延迟）
   - 本阶段目标
   - 待办清单（分 Batch）
   - 关键文件索引
   - 从延迟项中提取与本阶段相关的可选处理项

## 步骤 4：确认上下文

1. 向用户展示生成的 bootstrap 文档
2. 等待用户确认或调整
3. 确认后即可开始使用 `/refactor` 工作流执行改造
