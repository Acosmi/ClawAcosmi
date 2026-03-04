# Phase 1-2 审计修复报告

> 日期: 2026-02-12 | 范围: `internal/config`, `internal/infra`, `pkg/*`

## 修复统计

| 分类 | 数量 |
| ---- | ---- |
| P0 关键 Bug | 1 (loadFromDisk 管道跳过) |
| P1 逻辑 Bug | 2 (overrides 返回值 + schema 敏感字段) |
| P1 功能缺失 | 7 (defaults 默认值链补全) |
| 新增测试 | 22 |

## BUG-1~3 修复 (F1-F3)

### F1: `loadFromDisk` 跳过配置管道 (loader.go) — P0

**问题**: `loadFromDisk` 直接 JSON 解析，跳过 `$include`、环境变量替换、校验、默认值、覆盖 5 步管道。

**修复**: 提取 `applyConfigPipeline()` 共享方法，`loadFromDisk` 和 `ReadConfigFileSnapshot` 共用。

### F2: `mergeOverrides` 丢弃原始值 (overrides.go) — P1

**问题**: 返回类型 `map[string]interface{}` 无法表示原始值覆盖。

**修复**: 返回类型改为 `interface{}`，`ApplyConfigOverrides` 增加类型判断。

### F3: Sensitive 字段标记不生效 (schema.go) — P1

**问题**: `hints[f]` 前置 `ok` 检查导致新字段无法标记。

**修复**: 去除 `ok` 检查，直接创建 hint。

## BUG-4 修复 (F4a-F4g): defaults.go 默认值链补全

### 修复前

`defaults.go` (182L) vs TS `defaults.ts` (471L)，缺失 2 个函数 + 5 个空桩。

### 修复后 (285L)

| 子项 | 修复内容 | TS 对照行 |
|------|----------|-----------|
| F4a | `DefaultModelAliases` 4→6 条 | L14-26 |
| F4b | `applyMessageDefaults` → ackReactionScope | L113-126 |
| F4c | `applySessionDefaults` → mainKey: "main" | L128-152 |
| F4d | `applyLoggingDefaults` + redactSensitive | L335-350 |
| F4e | `applyCompactionDefaults` + mode: safeguard | L443-466 |
| F4f | `applyContextPruningDefaults` + heartbeat | L352-441 |
| F4g | `applyModelDefaults` (60L 新增) | L172-292 |

### 有意延迟项

| 函数 | 原因 | 预计 Phase |
|------|------|------------|
| `applyTalkApiKey` | 需 shell profile 读取 | Phase 3+ |
| Auth-mode heartbeat | 需 auth profile 解析 | Phase 3 |

## 验证

```
go build ./...   ✅
go vet ./...     ✅
go test ./...    ✅ (15 packages pass, 22 new tests)
```
