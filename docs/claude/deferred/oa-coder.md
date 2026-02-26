---
document_type: Deferred
status: In Progress
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-oa-coder-full.md, docs/claude/audit/audit-2026-02-26-oa-coder-completion-p1p2.md
skill5_verified: true
---

# oa-coder 延迟项

## 审计延迟修复 (LOW/INFO) — 已完成 ✅

- [x] F-06: Levenshtein 大输入 DoS — MAX_LEVENSHTEIN_INPUT = 10,000 + 快速拒绝
- [x] F-07: 二进制检测优化 — 只读前 8192 bytes
- [x] F-11: 非原子写入 — tempfile + rename (util/atomic.rs)
- [x] F-14: success_response 序列化失败 — match + error!() + -32603
- [x] F-17: rg --max-count per-file — 客户端计数截断
- [x] F-18: whitespace_normalized_replacer regex — 提取到循环外
- [x] F-19: filetime 模块未使用 — 已删除
- [x] F-21: jsonrpc 版本校验 — 添加 "2.0" 检查 + -32600
- [x] F-22: 空 old_string + 已存在文件 — 返回明确错误
- [x] F-23: 未使用 tokio 依赖 — 已从 [dependencies] 移除

## 可配置超时 — 已完成 ✅

- [x] Go F-03: 可配置超时 — CoderDefaultsConfig.TimeoutSeconds (默认 120s, 范围 [10,600])

## 功能延迟 — 移至主系统

- [→] codesearch tool — 移至主系统实现 (Go backend, 基于 GitHub REST API)
  - 主系统已有 web_fetch.go 基础设施，应在 attempt_runner 层面暴露给 agent
  - 不在 oa-coder Rust 侧重复实现
- [→] websearch tool — 移至主系统实现 (Go backend, 复用 web_fetch.go Brave/Perplexity)
  - 主系统已有完整 web_search + web_fetch 实现 (internal/agents/tools/web_fetch.go)
  - 仅需在 buildToolDefinitions() 中注册暴露给 agent
  - 不在 oa-coder Rust 侧重复实现

## 可视化重设计 — 终端式 UI (2026-02-26)

### 背景

原有可视化包含两部分:
1. **增强工具卡片** — coder 执行 edit/write/bash 等操作时，在聊天流中以卡片形式展示 diff 预览、bash 输出等
2. **确认弹窗** — coder 执行破坏性操作 (edit/write/bash) 前弹出 Allow/Deny 对话框，用户不审批则 60s 超时自动拒绝

用户评价"可视化卡片太丑了"，决定废弃当前卡片+弹窗方案，未来重设计为类似 Claude Code 的嵌入式终端体验。

### 禁用原因

用户明确要求: 禁用可视化，coder 子智能体仍可正常使用，可视化代码全部保留不删除。

### 当前运行状态

- **Coder bridge**: 正常启动，MCP 握手成功，6 个工具 (edit/read/write/grep/glob/bash) 全部注册可用
- **确认流程**: 已跳过。`CoderConfirmationManager` 未初始化 → `tool_executor.go` 中 `params.CoderConfirmation == nil` → 不触发确认请求 → edit/write/bash 直接执行，无审批
- **工具卡片**: coder 工具退化为普通工具卡片显示（与 web_fetch 等其他工具同级），不再展示 diff 预览等增强内容
- **确认弹窗**: 不再渲染，`renderCoderConfirmPrompt()` 调用已注释
- **安全影响**: coder 的 edit/write/bash 操作无需用户审批直接执行。用户已知晓并接受此风险

### 禁用点清单

搜索 `TODO(coder-terminal)` 可定位全部禁用点:

| # | 文件 | 行号 | 禁用方式 | 影响 | 恢复操作 |
|---|------|------|----------|------|----------|
| D1 | `backend/internal/gateway/boot.go` | ~200 | `CoderConfirmationManager` 初始化用 `/* */` 块注释包裹 | 确认管理器不创建，edit/write/bash 无需审批 | 取消 `/* */` 注释 |
| D2 | `ui/src/ui/chat/tool-cards.ts` | ~53-58 | `isCoderTool()` 增强渲染分支用 `//` 行注释 | coder 工具不再显示 diff 预览等增强卡片，退化为普通工具卡片 | 取消 `//` 注释 |
| D3 | `ui/src/ui/app-render.ts` | ~1424 | `renderCoderConfirmPrompt(state)` 替换为 `${""}` 空字符串 | 确认弹窗不再渲染 | 恢复为 `${renderCoderConfirmPrompt(state)}` |

### 保留的可视化代码清单

以下代码全部完好，恢复 D1-D3 后即可使用，零额外改动:

| 文件 | 职责 | LOC |
|------|------|-----|
| `ui/src/ui/chat/coder-tool-cards.ts` | 增强工具卡片渲染器 (edit diff 预览、bash 输出展示、write 内容预览) | ~242 |
| `ui/src/ui/views/coder-confirm.ts` | Allow/Deny 确认弹窗 + 倒计时 + 操作详情展示 | ~91 |
| `ui/src/styles/coder-cards.css` | 卡片 + 弹窗样式 | ~150 |
| `ui/src/ui/app-tool-stream.ts` | agent 事件 → 工具消息 → Lit reactive 渲染管线 | ~280 |
| `ui/src/ui/app-gateway.ts:354-372` | `coder.confirm.requested/resolved` WebSocket 事件路由 | ~18 |
| `backend/internal/agents/runner/coder_confirmation.go` | 后端确认管理器 (广播→channel 阻塞→超时/ctx 取消→自动 deny) | ~230 |

### 重设计方向

类似 Claude Code 的嵌入式终端体验:
- 执行任务时弹出或内嵌一个终端窗口，实时显示 coder 子智能体的执行过程 (命令、输出、文件变更)
- 替代当前的卡片式可视化，提供沉浸式的编程操作视觉反馈
- 确认交互也在终端内完成（内联 prompt 而非独立弹窗）
- 设计与实现由用户发起，当前仅记录方向

### 恢复旧版可视化的步骤

1. 搜索 `TODO(coder-terminal)`，按 D1-D3 表格恢复 3 处注释/替换
2. 重新编译 Go backend + 前端
3. 重启 gateway → 确认日志出现 `"gateway: coder confirmation manager initialized"`
4. 发起 coder 任务 → 应看到增强工具卡片 + edit/write/bash 确认弹窗

## 仍待做

- [ ] **终端式 coder UI 设计与实现** — 待用户发起，当前仅记录方向
- [ ] boot.go CLI args 传递 — coder 进程启动参数 (--sandboxed, --workspace) 需从 config 读取
  - 当前 boot.go 在 config 加载前运行，需重构初始化顺序或延迟启动
- [ ] 独立开源包装 (README, LICENSE, CI)
- [ ] 前端配置向导 (wizard-coder.ts) — 依赖 codesearch/websearch 最终方案确定
- [ ] Schema UI Hints — 依赖 CoderDefaultsConfig 最终字段确定
