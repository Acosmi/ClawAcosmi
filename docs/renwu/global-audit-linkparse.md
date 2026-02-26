# linkparse 全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W8

## 概览

| 维度 | TS (`src/link-understanding`) | Go (`backend/internal/linkparse`) | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 6 | 5 | 83% |
| 总行数 | 268 | 414 | 154% |

## 逐文件对照

| 状态 | 文件 | 对应关系 | 备注 |
|------|------|----------|------|
| ✅ FULL | `defaults.ts` | `defaults.go` | 常量完全一致 |
| ✅ FULL | `detect.ts` | `detect.go` | URL提取、剥离Markdown语法、过滤逻辑完全一致 |
| ✅ FULL | `format.ts` | `format.go` | 拼接输出和格式化一致 |
| ⚠️ PARTIAL | `runner.ts` | `runner.go` | Go版本中替换CLI参数时，仅简单替换 `{{LinkUrl}}`，而未完整实现 TS 的 `applyTemplate(part, templCtx)` 引擎解析。 |
| ⚠️ PARTIAL | `apply.ts` | `apply.go` | TS 版将输出追加至 `ctx.LinkUnderstanding` 数组，Go版遗漏了该操作，仅处理了 `Body` 追加。 |
| ✅ FULL | `index.ts` | N/A | Go 包级暴露，无需此文件 |

## 隐藏依赖审计

| # | 类别 | 检查方法与结果 |
|---|------|---------|
| 1 | npm 包黑盒行为 | `grep node_modules` -> **无**。未使用第三方黑盒库。 |
| 2 | 全局状态/单例 | `grep global...` -> **有**。发现引用了 `globals.js` 中的 `logVerbose`。Go 版中已作为 `verbose` 参数传入，并使用标准 `log` 包替代。 |
| 3 | 事件总线/回调链 | `grep EventEmitter...` -> **无**。 |
| 4 | 环境变量依赖 | `grep process.env` -> **无**。 |
| 5 | 文件系统约定 | `grep fs...` -> **无**。 |
| 6 | 协议/消息格式 | CLI 标准输出截取最大 `1MB` (在 Go 中 `CLIOutputMaxBuffer` 定义) |
| 7 | 错误处理约定 | `grep catch...` -> **有**。`detect.ts` 捕获 URL 解析错误，`runner.ts` 捕获 CLI 失败。均有安全的容错设计。Go版中分别用 `err != nil` 处理，逻辑等价。 |

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| LINK-1 | 功能遗漏 | `apply.ts` | `apply.go` | Go 版本在 `ApplyLinkUnderstanding` 中缺少对 `params.Ctx.LinkUnderstanding` 数组的写入和合并，仅合并了 `Body`。 | **P1** | 在 `apply.go` 中补充 `params.Ctx.LinkUnderstanding = append(params.Ctx.LinkUnderstanding, result.Outputs...)`。 |
| LINK-2 | 功能简化 | `runner.ts` | `runner.go` | `RunCliEntry` 时，TS版使用了通用的 `applyTemplate` 解析工具替换所有上下文变量，而 Go 版仅使用了简单的字符串替换 `{{LinkUrl}}`。 | **P2** | 确认为有意简化或属于推迟项。如果未来需要在链接理解的CLI中注入更多变量，需要引入模板引擎。 |

## 总结

- P0 差异: 0 项
- P1 差异: 1 项
- P2 差异: 1 项
- 模块审计评级: **B** (由于存在少量上下文变量遗漏)
