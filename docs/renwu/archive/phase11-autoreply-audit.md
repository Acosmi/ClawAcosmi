# 模块 C: AutoReply 管线 — 重构健康度审计报告

> 审计日期: 2026-02-17
> 方法论: `/refactor` 六步循环法 + 隐藏依赖审计

---

## 一、概述

| 维度 | TS 原版 | Go 移植 | 覆盖率 |
|------|---------|---------|--------|
| 源文件数 (root) | 21 | 31 | — |
| 源文件数 (reply/) | 98 | 34 | 34.7% |
| 总代码行 | 22,028 | 11,145 | 50.6% |
| 测试行数 | — | 3,539 (23 文件) | — |

**编译状态**: `go build` ✅ `go vet` ✅ `go test -race` ✅ (2 包全部通过)

---

## 二、根目录文件映射 (TS → Go)

| TS 文件 | 行数 | Go 文件 | 行数 | 状态 |
|---------|------|---------|------|------|
| `dispatch.ts` | 77 | `dispatch.go` | 28 | ⚠️ 仅类型定义，无分发逻辑 |
| `envelope.ts` | 219 | `envelope.go` | 66 | ⚠️ 缺时区/elapsed/sender-label |
| `status.ts` | 679 | `status.go` | 384 | ⚠️ 缺 readUsageFromSessionLog / formatVoiceModeLine |
| `commands-registry.ts` | 520 | `commands_registry.go` | 232 | ⚠️ 缺 text alias / config 过滤 / arg choice |
| `commands-registry.data.ts` | 614 | `commands_data.go` | 212 | ⚠️ ~65% 命令缺失 |
| `chunk.ts` | 500 | `chunk.go` | 447 | ✅ 基本完整 |
| `heartbeat.ts` | 157 | `heartbeat.go` | 190 | ✅ 完整 |
| `thinking.ts` | 233 | `thinking.go` | 345 | ✅ 完整 |
| `templating.ts` | 192 | `templating.go` | 133 | ⚠️ 缺模板渲染部分函数 |
| `command-auth.ts` | 270 | `command_auth.go` | 70 | ⚠️ 大幅缩减 |
| `skill-commands.ts` | 141 | `skill_commands.go` | 190 | ✅ 完整 |
| `inbound-debounce.ts` | 110 | `inbound_debounce.go` | 89 | ✅ 基本完整 |
| `send-policy.ts` | 44 | `send_policy.go` | 64 | ✅ 完整 |
| `command-detection.ts` | 88 | `command_detection.go` | 61 | ✅ 完整 |
| `group-activation.ts` | 34 | `group_activation.go` | 69 | ✅ 完整 |
| `media-note.ts` | 93 | `media_note.go` | 56 | ✅ 基本完整 |
| `commands-args.ts` | 100 | `commands_args.go` | 121 | ✅ 完整 |
| `model.ts` | 52 | `model.go` | 95 | ✅ 完整 |
| `types.ts` | 59 | `types.go` | 59 | ✅ 完整 |
| `tokens.ts` | 22 | `tokens.go` | 50 | ✅ 完整 |
| `tool-meta.ts` | 143 | `tool_meta.go` | 60 | ⚠️ 缩减 |

## 三、reply/ 子目录已有文件

| Go 文件 | 行数 | TS 对应 | 行数 | 状态 |
|---------|------|---------|------|------|
| `abort.go` | 198 | `abort.ts` | 205 | ✅ |
| `agent_runner.go` | 162 | `agent-runner.ts` | 525 | ⚠️ 缩减 69% |
| `agent_runner_execution.go` | 96 | 604L | ⚠️ 缩减 84% |
| `agent_runner_memory.go` | 88 | 202L | ⚠️ 缩减 56% |
| `agent_runner_payloads.go` | 117 | 121L | ✅ |
| `agent_runner_utils.go` | 225 | 136L | ✅ 扩展 |
| `body.go` | 109 | `body.ts` 50L | ✅ 扩展 |
| `directive_parse.go` | 232 | 215L | ✅ |
| `directive_persist.go` | 140 | 246L | ⚠️ 缩减 43% |
| `directive_shared.go` | 94 | 66L | ✅ |
| `directives.go` | 228 | 193L | ✅ |
| `dispatch_from_config.go` | 148 | 458L | ⚠️ 缩减 68% |
| `exec_directive.go` | 119 | `exec/directive.ts` 230L | ⚠️ 缩减 48% |
| `followup_runner.go` | 101 | 285L | ⚠️ 缩减 65% |
| `get_reply.go` | 204 | 335L | ⚠️ 缩减 39% |
| `get_reply_directives.go` | 281 | 488L | ⚠️ 缩减 42% |
| `get_reply_directives_apply.go` | 195 | 314L | ⚠️ 缩减 38% |
| `get_reply_directives_utils.go` | 12 | 47L | ⚠️ 缩减 74% |
| `get_reply_inline_actions.go` | 185 | 384L | ⚠️ 缩减 52% |
| `get_reply_run.go` | 181 | 434L | ⚠️ 缩减 58% |
| `history.go` | 159 | 193L | ✅ 基本完整 |
| `inbound_context.go` | 254 | 81L | ✅ 扩展 |
| `memory_flush.go` | 159 | 105L | ✅ 扩展 |
| `mentions.go` | 136 | 157L | ✅ 基本完整 |
| `model_fallback_executor.go` | 224 | — | ✅ Go 新增 |
| `normalize_reply.go` | 84 | 94L | ✅ |
| `queue_directive.go` | 138 | `queue/directive.ts` 196L | ⚠️ 缺队列管理 |
| `reply_dispatcher.go` | 227 | 193L | ✅ |
| `reply_inline.go` | 76 | 41L | ✅ |
| `response_prefix.go` | 21 | 101L | ⚠️ 缩减 79% |
| `route_reply.go` | 210 | 162L | ✅ |
| `typing.go` | 284 | 196L | ✅ 扩展 |
| `typing_mode.go` | 149 | 142L | ✅ |
| `types.go` | 42 | — | ✅ Go 新增 |

## 四、reply/ 中 TS 独有文件 (Go 端无对应)

### P0 — 核心管线缺失 (~2,240L)

| TS 文件 | 行数 | 职责 | 影响 |
|---------|------|------|------|
| `session.ts` | 393 | SessionManager 集成 (依赖 npm 包) | 🔴 会话持久化核心 |
| `session-updates.ts` | 275 | 会话元数据更新 | 🔴 turn 计数/时间戳/usage |
| `model-selection.ts` | 584 | 模型选择+回退逻辑 | 🔴 模型切换核心 |
| `bash-command.ts` | 425 | Bash 工具执行管线 | 🟡 已有 agents/bash-tools |
| `session-usage.ts` | 103 | 会话用量统计 | 🔴 cost 追踪 |
| `session-reset-model.ts` | 202 | 会话重置+模型 Gemini 修复 | 🟡 |
| `reply-elevated.ts` | 233 | 提权回复管线 | 🟡 安全控制 |

### P1 — 流式/队列子系统 (~1,233L)

| TS 文件 | 行数 | 职责 |
|---------|------|------|
| `queue/types.ts` | 90 | 队列类型定义 |
| `queue/state.ts` | 76 | 队列状态管理 |
| `queue/enqueue.ts` | 69 | 入队逻辑 |
| `queue/drain.ts` | 135 | 出队/排干 |
| `queue/directive.ts` | 196 | 队列指令解析 |
| `queue/normalize.ts` | 44 | 队列模式规范化 |
| `queue/settings.ts` | 68 | 队列设置解析 |
| `queue/cleanup.ts` | 29 | 队列清理 |
| `block-reply-pipeline.ts` | 242 | 块流式回复管线 |
| `block-streaming.ts` | 165 | 流式分块合并 |
| `block-reply-coalescer.ts` | 147 | 回复块合并器 |
| `streaming-directives.ts` | 128 | 流式指令 |

### P2 — 辅助功能 (~1,700L)

| TS 文件 | 行数 | 职责 |
|---------|------|------|
| `line-directives.ts` | 342 | LINE 频道指令 |
| `commands-allowlist.ts` | 695 | 命令白名单管理 |
| `commands-subagents.ts` | 430 | 子代理命令 |
| `reply-payloads.ts` | 123 | 回复载荷构建 |
| `reply-threading.ts` | 63 | 线程回复 |
| `reply-reference.ts` | 62 | 引用回复 |
| `stage-sandbox-media.ts` | 197 | 沙箱媒体暂存 |

### P3 — 已在其他位置实现或可忽略 (~470L)

| TS 文件 | 行数 | 说明 |
|---------|------|------|
| `test-ctx.ts` | 17 | 测试辅助 |
| `test-helpers.ts` | 18 | 测试辅助 |
| `audio-tags.ts` | 1 | 重导出 |
| `exec.ts` | 1 | 重导出 |
| `queue.ts` | 14 | 重导出 |
| `commands.ts` | 8 | 重导出 |
| `directive-handling.ts` | 6 | 重导出 |
| `inbound-text.ts` | 3 | 重导出 |
| `untrusted-context.ts` | 16 | 类型定义 |
| `provider-dispatcher.ts` | 44 | 分发器 (Go 端已内联) |
| `inbound-dedupe.ts` | 55 | 去重 (Go 端 `inbound_context.go`) |
| `inbound-sender-meta.ts` | 57 | 发送者元数据 (Go 端 `inbound_context.go`) |
| `groups.ts` | 133 | 群组逻辑 (Go 端 `group_activation.go`) |
| `reply-tags.ts` | 22 | 标签 (Go 端内联) |
| `config-commands.ts` | 71 | 配置命令 (Go 端 `commands_handler_config.go`) |
| `config-value.ts` | 48 | 配置值 (Go 端内联) |
| `debug-commands.ts` | 72 | 调试命令 (功能已覆盖) |
| `commands-types.ts` | 64 | 类型 (Go 端 `commands_handler_types.go`) |
| `subagents-utils.ts` | 32 | 子代理工具 (Go 端 `commands_handler_subagents.go`) |

---

## 五、隐藏依赖审计 (7 项检查)

| # | 类别 | 结果 | 说明 |
|---|------|------|------|
| 1 | npm 包黑盒行为 | ⚠️ | `session.ts` 依赖 `@mariozechner/pi-coding-agent` SessionManager — Go 端已用 `internal/session/` 替代 |
| 2 | 全局状态/单例 | ⚠️ | `commands-registry.ts` 有 4 个模块级缓存变量 (`cachedTextAliasMap` 等) — Go 端用 `globalCommands` map 替代，缺缓存失效 |
| 3 | 事件总线/回调链 | ✅ | 仅 `stage-sandbox-media.ts` 有 child.stderr `.on()` 事件；Go 端已在 `agents/bash-tools` 覆盖 |
| 4 | 环境变量依赖 | ✅ | 仅 `get-reply.ts` 读 `OPENACOSMI_TEST_FAST`（测试用），非生产路径 |
| 5 | 文件系统约定 | ⚠️ | `session.ts` 依赖磁盘 JSON 文件读写会话；Go 端用内存 Map (模块 B 审计已记录，P0 优先) |
| 6 | 协议/消息格式 | ✅ | 消息格式由 `envelope.go` / `templating.go` 控制，基础格式一致 |
| 7 | 错误处理约定 | ⚠️ | TS 端 `session.ts` 有 try/catch 的 JSON 解析降级；Go 端内存模型无此需求，但缺 corruption recovery |

### 隐藏依赖汇总

- **⚠️ 项共 4 个**：#1 npm 包、#2 全局缓存、#5 文件 I/O、#7 错误降级
- **#1 和 #5** 已在模块 B 审计中作为 P0 记录，会话存储架构差异是跨模块问题
- **#2** 影响低，Go 没有热重载需求，全局 map 在进程生命周期内稳定
- **#7** Go 内存模型不存在 JSON 解析失败场景，可接受

---

## 六、关键行为偏差分析

### 6.1 dispatch 入口 (P1)

- **TS**: `dispatchInboundMessage()` 完整管线 — finalize context → dispatchReplyFromConfig → typing dispatcher
- **Go**: `dispatch.go` 仅定义 `DispatchOutcome` 类型，实际逻辑在 `reply/dispatch_from_config.go`
- **结论**: 分发逻辑存在但分散，缺少顶层统一入口。功能基本可用

### 6.2 envelope 封装 (P2)

- **TS**: 219L — 完整时区解析 (local/utc/user/IANA) + elapsed time + sender label
- **Go**: 66L — 仅基础时间戳 + 简单 sender label
- **缺失**: `resolveEnvelopeTimezone()` 4 模式、`formatTimeAgo()` elapsed、`resolveSenderLabel()` 富标签

### 6.3 commands-registry (P1)

- **TS**: 520L — text alias 缓存 + config 过滤 + arg choice resolver + native name overrides
- **Go**: 232L — 核心注册/查找/解析完整
- **缺失**: `listChatCommandsForConfig()` 按配置过滤、`resolveCommandArgChoices()` 参数选项、text alias 映射缓存

### 6.4 commands-data (P1)

- **TS**: 614L — ~30 命令定义
- **Go**: 212L — ~12 命令定义
- **缺失命令估计**: ~18 个命令定义未注册 (部分为频道特定如 LINE)

### 6.5 status (P2)

- **TS**: 679L — 完整 status 消息 + help 消息 + 分页命令列表
- **Go**: 384L — 核心 status + help 已实现
- **缺失**: `readUsageFromSessionLog()` 会话日志用量读取、`formatVoiceModeLine()` TTS 状态、`buildCommandsMessagePaginated()` 分页

### 6.6 reply/ 管线深度 (P0)

- **TS reply/**: 98 文件 — 完整 session 管理 + followup 队列 + 模型选择 + block streaming + elevated 回复
- **Go reply/**: 34 文件 — DI 骨架 + 核心指令解析 + agent runner 集成
- **核心缺口**: queue/* followup 系统 (8 文件 679L)、session 管理 (4 文件 973L)、model-selection (584L)

---

## 七、优先行动计划

### P0 — 影响运行时正确性

| # | 项目 | 估计工作量 | 说明 |
|---|------|-----------|------|
| C-P0-1 | model-selection.ts (584L) | 1 窗口 | 模型切换/回退核心，当前依赖 `model_fallback_executor.go` 的 DI |
| C-P0-2 | session 管理 4 文件 (973L) | 1 窗口 | session.ts + updates + usage + reset-model，依赖模块 B 的 session store 架构 |
| C-P0-3 | dispatch 顶层入口 | 0.5 窗口 | 补全 `dispatch.go` 统一分发逻辑 |

### P1 — 影响功能完整性

| # | 项目 | 估计工作量 |
|---|------|-----------|
| C-P1-1 | queue/* followup 系统 (679L) | 1 窗口 |
| C-P1-2 | commands-data 缺失命令 (~18 个) | 0.5 窗口 |
| C-P1-3 | commands-registry 缺失函数 | 0.5 窗口 |
| C-P1-4 | block-streaming 管线 (554L) | 1 窗口 |

### P2 — 影响用户体验

| # | 项目 | 估计工作量 |
|---|------|-----------|
| C-P2-1 | envelope 完整时区/elapsed | 0.5 窗口 |
| C-P2-2 | command-auth 权限检查 | 0.5 窗口 |
| C-P2-3 | reply-elevated 提权管线 | 0.5 窗口 |
| C-P2-4 | status 缺失格式化函数 | 0.5 窗口 |
| C-P2-5 | LINE 指令 + 线程回复 | 0.5 窗口 |

### P3 — 延迟至后续阶段

- bash-command.ts — 已有 `agents/bash-tools` 覆盖大部分
- stage-sandbox-media.ts — 沙箱媒体暂存非核心路径
- reply-reference.ts — 引用回复辅助

---

## 八、验证结果

```
$ cd backend && go build ./...       # ✅ 通过
$ cd backend && go vet ./...         # ✅ 通过
$ cd backend && go test -race ./internal/autoreply/...
ok  autoreply       1.019s
ok  autoreply/reply  1.071s           # ✅ 全部通过
```
