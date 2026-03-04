# 模块 F: Config/Scope — 审计 Bootstrap

> 用于新窗口快速恢复上下文

---

## 新窗口启动模板

```
请执行 Config/Scope 模块的重构健康度审计。

## 上下文
1. 读取审计总表: `docs/renwu/refactor-health-audit-task.md`
2. 读取本 bootstrap: `docs/renwu/phase11-config-scope-bootstrap.md`
3. 读取 `/refactor` 技能工作流
4. 读取编码规范: `skills/acosmi-refactor/references/coding-standards.md`
5. 读取 `docs/renwu/deferred-items.md`
6. 控制输出量：预防上下文过载引发崩溃，需要大量输出时请逐步分段输出。
7. 任务完成后：请按要求更新 `refactor-plan-full.md` 和本模块的审计报告。

## 目标
对比 TS 原版 `src/config/` + `src/agents/agent-scope.ts` 与 Go 移植 `backend/internal/agents/scope/` + `backend/internal/config/`。

> **注意**: 具体审计步骤请严格参考 `docs/renwu/refactor-health-audit-task.md` 模块 F 章节。此文档仅提供上下文和文件索引。
```

---

## TS 源文件 (核心, 按重要性排序)

| 文件 | 大小 | 职责 |
|------|------|------|
| `schema.ts` | 56KB | ⭐⭐ 配置 schema (最大文件) |
| `zod-schema.ts` | 20KB | Zod 校验 schema |
| `zod-schema.providers-core.ts` | 30KB | Provider 配置校验 |
| `zod-schema.core.ts` | 16KB | 核心配置校验 |
| `io.ts` | 19KB | ⭐ 配置文件读写 |
| `defaults.ts` | 12KB | ⭐ 默认值填充 |
| `validation.ts` | 11KB | 校验逻辑 |
| `types.tools.ts` | 16KB | 工具类型定义 |
| `plugin-auto-enable.ts` | 12KB | 插件自动启用 |
| `paths.ts` | 9KB | 路径解析 |
| `includes.ts` | 7KB | 配置文件包含机制 |
| `types.gateway.ts` | 8KB | Gateway 类型 |
| `group-policy.ts` | 6KB | 组策略 |
| `redact-snapshot.ts` | 6KB | 快照脱敏 |
| `agent-scope.ts` (agents/) | 6KB | ⭐ Agent 作用域解析 |

## Go 对应文件

| 目录/文件 | 文件数 | 对应 TS |
|-----------|--------|---------|
| `agents/scope/` | 8 文件 | `agent-scope.ts` + 部分 `defaults.ts` |
| `backend/internal/config/` | 如存在 | `io.ts` + `schema.ts` + `validation.ts` |
| `gateway/hooks_mapping.go` | 16KB | 部分配置映射 |
| `gateway/reload.go` | 13KB | 配置热重载 |

## 关键审计点

1. **Schema 缺失**: 56KB `schema.ts` 是否有 Go 等价？(Zod → Go 校验？)
2. **配置 I/O**: `io.ts` 18KB 处理 YAML 读写、合并、包含，Go 端如何实现？
3. **默认值填充**: `defaults.ts` 12KB 复杂的默认值逻辑
4. **Agent Scope**: `resolveDefaultAgentId()` 等关键函数一致性
5. **环境变量替换**: `env-substitution.ts` 在配置值中替换 `${ENV_VAR}`
6. **Legacy 迁移**: 3 个 `legacy.migrations.*.ts` 共 33KB，是否仍需移植？
7. **插件自动启用**: `plugin-auto-enable.ts` 12KB，Go 端是否实现？

## 注意事项

> [!WARNING]
> `src/config/` 是最复杂的模块 (124 文件)，许多文件可能是渠道特定配置
> (Discord、Slack、Telegram 等)。如果 Go 移植仅支持 Gateway 模式，
> 这些渠道配置可能不需要移植。审计时应先判定哪些配置路径是 Gateway 模式必须的。
