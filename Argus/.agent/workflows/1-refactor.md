---
description: Go+Rust 混合架构重构执行工作流。用于启动一轮新的重构任务，涵盖上下文加载、计划编写、代码执行、验证和文档更新的完整流程。
---

# /refactor — 重构执行工作流

// turbo-all

## 步骤 0：加载技能与规范

1. 读取技能入口：`.agent/skills/acosmi-refactor/SKILL.md`
2. 读取编码规范：`.agent/skills/acosmi-refactor/references/coding-standards.md`
3. 读取 FFI 规范：`.agent/skills/acosmi-refactor/references/ffi-conventions.md`
4. 读取重构排序：`.agent/skills/acosmi-refactor/references/refactor-order.md`

## 步骤 1：加载上下文

1. 读取当前阶段的 bootstrap 文档：`docs/renwu/bootstrap-<phase>.md`（如不存在则先执行 `/bootstrap`）
2. 读取任务跟踪文件：`docs/renwu/task-YYYYMMDD-<topic>.md`（如 bootstrap 中有指定）
3. 读取架构方案：`docs/jiagou/改造Go+Rust 混合架构.md`
4. 读取延迟待办：`docs/renwu/deferred-items.md`（如存在）
5. 向用户确认本次要处理的具体任务范围

## 步骤 2：编写实施计划

1. 参照文档模板（`.agent/skills/acosmi-refactor/references/doc-template.md`）编写任务计划
2. 计划中应包含：背景与目标、具体变更文件列表、验证方式、风险说明
3. 遵守工作量控制：并行修改文件 ≤ 5，涉及模块 ≤ 2，步骤 ≤ 4
4. **等待用户审批后再继续**

## 步骤 3：执行代码变更

1. 按计划逐步执行代码变更
2. 遵守编码规范和 FFI 规范
3. 每完成一个步骤后运行基础编译检查：
   - Rust: `cargo build --release`（在 `rust-core/` 目录）
   - Go: `go build ./...`（在 `go-sensory/` 目录）
4. 每完成一个 Batch/步骤后，更新 `task-*.md` 中对应项的状态标记：
   - ⬜ → 🔄 进行中
   - 🔄 → ✅ 完成
   - 跳过的标记为 ⏭️ 并说明原因

## 步骤 4：验证

1. 运行完整编译：`cargo build --release` + `go build ./...`
2. 运行静态分析：`go vet ./...` + `cargo clippy`
3. 运行相关单元测试
4. 如涉及 web-console 端点，启动服务验证兼容性

## 步骤 5：更新文档

任务完成后，逐一更新以下文档：

1. 更新任务跟踪：`docs/renwu/task-YYYYMMDD-<topic>.md`
   - 所有已完成项标记为 ✅
   - 勾选"完成后需更新的文档"清单
   - 更新变更记录
2. 更新审计/来源报告：如修复项来自审计，在审计报告中标记已修复
3. 更新 bootstrap 文档：标记已完成的阶段
4. 更新延迟待办：`docs/renwu/deferred-items.md`（登记跳过/延迟项）
5. 更新受影响模块的架构文档：`docs/gouji/<module>.md`（如有结构变更）
6. 向用户汇报完成情况
