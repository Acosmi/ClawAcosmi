---
document_type: Deferred
status: Draft
created: 2026-02-25
last_updated: 2026-02-25
audit_report: Pending
skill5_verified: false
---

# Argus 视觉子智能体技能迁移至 docs/skills/

## 背景

Argus 视觉理解执行子智能体的技能目前通过代码硬编码注入：
- `server_methods_skills.go` → `argus.BuildArgusSkillEntries(bridge.Tools())`
- 不走 SKILL.md 文件系统，与其他技能管理方式不统一

## 当前状况

- Argus 工具定义在 `internal/argus/` 包中，由 MCP bridge 动态提供
- 技能状态 API (`skills.status`) 已经能展示 Argus 技能（source="argus"）
- 但 LLM prompt 注入路径与文件系统技能分离

## 建议方案

1. 在 `docs/skills/` 下新增 `argus/` 分类目录
2. 为每个 Argus 工具创建 SKILL.md（screenshot, click, type, navigate 等）
3. 修改 `BuildArgusSkillEntries` 优先从 SKILL.md 读取描述，代码工具定义仅作 fallback
4. 统一注入路径：所有技能通过 `LoadSkillEntries` → `SkillsPrompt` → LLM

## 影响

- 用户可通过编辑 SKILL.md 自定义 Argus 工具描述
- 技能管理统一入口：增删改都在 `docs/skills/argus/`
- 需处理动态工具（MCP bridge 工具列表可能变化）与静态 SKILL.md 的同步

## 优先级

低 — 当前硬编码方式功能正常，迁移为纯架构优化。

## 前置条件

- docs/skills/ 文件系统技能扫描已完成 ✅
- SkillsPrompt 注入已修复 ✅
