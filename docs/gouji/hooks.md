# hooks/ 架构文档

> 最后更新：2026-02-26 | 代码级审计完成

## 一、模块概述

| 属性 | 值 |
| ---- | ---- |
| 模块路径 | `backend/internal/hooks/` |
| Go 源文件数 | 18 (含 gmail/ 子包) |
| Go 测试文件数 | 3 |
| 测试函数数 | 35 |
| 总行数 | ~4,230 |

钩子系统：HOOK.md 解析、加载/发现、内部事件、Gmail 集成、Soul Evil 覆盖、插件钩子、LLM Slug 生成。

## 二、文件索引

### 核心钩子 (12 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `hook_types.go` | ~145 | Hook 类型定义 |
| `hook_config.go` | ~180 | Hook 配置解析 |
| `hook_install.go` | — | 单钩子安装逻辑 |
| `hook_installs.go` | — | 批量钩子安装管理 |
| `hooks.go` | — | 钩子系统主入口 |
| `frontmatter.go` | ~150 | HOOK.md frontmatter 解析 |
| `workspace.go` | ~200 | 工作区钩子发现 |
| `loader.go` | ~120 | 钩子加载器 |
| `bundled_dir.go` | ~60 | 内置钩子目录 |
| `bundled_handlers.go` | ~80 | 内置钩子处理器 |
| `status.go` | ~100 | 钩子状态管理 |
| `internal_hooks.go` | ~200 | 内部事件钩子 |

### 扩展模块 (4 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `soul_evil.go` | ~300 | SOUL.md 条件覆盖 (时区+概率) |
| `plugin_hooks.go` | — | 插件钩子集成 |
| `llm_slug_generator.go` | — | LLM slug 命名生成 |

### gmail/ 子包 (4 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `gmail/gmail.go` | ~340 | Gmail 配置+解析 |
| `gmail/ops.go` | ~195 | Setup 向导 |
| `gmail/setup.go` | ~200 | gcloud/tailscale CLI |
| `gmail/watcher.go` | ~230 | Watcher 进程生命周期 |

## 三、测试覆盖

| 测试文件 | 测试数 | 覆盖范围 |
|----------|--------|----------|
| `hook_config_test.go` | — | 配置解析 |
| `hooks_test.go` | — | 钩子匹配 |
| `internal_hooks_test.go` | — | 内部事件 |
| **合计** | **35** | |
