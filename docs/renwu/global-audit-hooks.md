# hooks 全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W8，部分 2 (中型模块)

## 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 22 | 19 | 86.3% |
| 总行数 | 3914 | 4703 | 120.1% |

*(注：Go端将一些零散的小型内置handler整合到 `bundled_handlers.go` 中，使得文件数略少，结构更紧凑)*

## 逐文件对照

| 状态 | TS 文件 | Go 文件 |
|------|---------|---------|
| ✅ FULL | `hooks.ts` | `hooks.go` |
| ✅ FULL | `installs.ts`, `install.ts` | `hook_installs.go`, `hook_install.go` |
| ✅ FULL | `bundled-dir.ts` | `bundled_dir.go` |
| ✅ FULL | `types.ts` | `hook_types.go` |
| ✅ FULL | `llm-slug-generator.ts` | `llm_slug_generator.go` |
| ✅ FULL | `plugin-hooks.ts` | `plugin_hooks.go` |
| ✅ FULL | `loader.ts` | `loader.go` |
| ✅ FULL | `frontmatter.ts` | `frontmatter.go` |
| ✅ FULL | `config.ts` | `hook_config.go` |
| ✅ FULL | `internal-hooks.ts` | `internal_hooks.go` |
| ✅ FULL | `hooks-status.ts` | `status.go` |
| ✅ FULL | `gmail-watcher.ts`, `gmail.ts`, `gmail-ops.ts`, `gmail-setup-utils.ts` | `gmail/watcher.go`, `gmail/gmail.go`, `gmail/ops.go`, `gmail/setup.go` |
| ✅ FULL | `soul-evil.ts` | `soul_evil.go` |
| ✅ FULL | `workspace.ts` | `workspace.go` |
| ✅ FULL | `bundled/*/handler.ts` (boot-md, soul-evil, command-logger, session-memory) | `bundled_handlers.go` |

> 评价：Go端实现了极高质量的文件粒度对齐，不仅分离了类型(`hook_types.go`)和配置(`hook_config.go`)，还将所有 `gmail` 相关的逻辑打包到单独的子包 `gmail/` 中，这是比 TS 原版更清晰的架构。

## 隐藏依赖审计

| # | 类别 | 检查结果 | Go端实现方案 |
|---|------|----------|-------------|
| 1 | npm 包黑盒行为 | ✅ 无严重依赖 | Go端自主实现或调用基础库 |
| 2 | 全局状态/单例 | ✅ 无 | Hooks 服务均通过依赖注入完成，避免独立单例 |
| 3 | 事件总线/回调链 | ⚠️ TS端注册部分回调 | Go端利用 Interface(`Hook`) 和结构体方法注册触发点 |
| 4 | 环境变量依赖 | ✅ 无直接依赖 | 所有的环境变量和配置均通过 Gateway Context 或配置管理器传递 |
| 5 | 文件系统约定 | ⚠️ `bundled-dir.ts` 及 `loader.ts` 存在目录扫描 | Go端统一使用 `os` 和 `filepath` 标准包，针对不同系统平台处理路径。 |
| 6 | 协议/消息格式 | ✅ 无特殊 | 只涉及内部的数据流流转 |
| 7 | 错误处理约定 | ⚠️ 标准 Error 抛出 | Go端使用了具体的错误类型区分如 HookTimeout, HookInitError |

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| HOOK-1 | 架构重构 | `bundled/*/handler.ts` | `bundled_handlers.go` | Go端将所有极小的预置 Hook handler 打包成一个 go 文件，而非分散的多个深层目录，减少了碎片。 | P3 | 优秀的代码整理，无需修复 |
| HOOK-2 | 包组织 | `gmail-*.ts` | `gmail/` 目录 | Gmail 系列的 TS 文件直接在 root hooks 目录下，而 Go 将它们收集进独立的 `gmail` 模块下保护作用域。 | P3 | 更好的可见性控制，无需修复 |

## 总结

- P0 差异: 0 项
- P1 差异: 0 项
- P2 差异: 0 项
- **模块审计评级: A** (模块组织上 Go 版本明显优于 TS 版本，逻辑平移完美对接)
