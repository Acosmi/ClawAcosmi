---
document_type: Audit
status: Complete
created: 2026-02-26
last_updated: 2026-02-26
scope: UHMS 上下文压缩技能 + 压缩管线升级
verdict: PASS (1 MEDIUM fixed, 4 INFO noted)
---

# 审计报告: UHMS 压缩管线升级

## 审计范围

| 文件 | 操作 | 审计行 |
|---|---|---|
| `uhms/compression_prompts.go` | NEW | 全文 (119 行) |
| `uhms/config.go` | MOD | L104-117 (3 字段), L163-179 (2 访问器) |
| `types/types_memory.go` | MOD | L92-94 (3 字段) |
| `gateway/server.go` | MOD | L388-396 (3 映射) |
| `uhms/manager.go` | MOD | L53-55 (lastSummary), L330-378 (maskObservations), L384-494 (CompressIfNeeded), L938-982 (summarizeMessages), L671/1002/839/872 (prompt 常量) |
| `uhms/claude_integration.go` | MOD | L274-276 (summarizeWithLocalLLM prompt) |
| `uhms/session_committer.go` | MOD | L97/106/109/147 (4 处 prompt 常量) |
| `docs/skills/tools/context-compressor/SKILL.md` | NEW | 技能文件 |

## 发现清单

### F-01 [MEDIUM] maskObservations: cutoffIdx 默认值导致全量遮蔽 — **已修复**

**位置**: `manager.go:maskObservations()`

**问题**: 原代码 `cutoffIdx := len(messages)` 注释写 "default: mask nothing"，但实际效果是 "mask everything"。当 user turn 数量 < `maskTurns` 时（如 2 个 user turn 但 `maskTurns=3`），`cutoffIdx` 不被更新，for 循环遍历全部消息，导致所有 tool/system 输出被错误遮蔽。

**修复**: 将默认值改为 `cutoffIdx := 0`（真正的 mask nothing），条件改为 `cutoffIdx <= 0` 返回原消息。

**验证**: 编译通过。逻辑分析:
- `[user, tool, tool, user, tool]` + maskTurns=1: cutoffIdx=3, 遮蔽 0-2 ✅
- `[user]` + maskTurns=1: cutoffIdx=0, 返回原消息 ✅
- `[tool, tool]` + maskTurns=1: 无 user turn, cutoffIdx=0, 返回原消息 ✅
- `[user, tool]` + maskTurns=5: 仅 1 个 user turn < 5, cutoffIdx=0, 返回原消息 ✅

### F-02 [INFO] summarizeMessages: Compaction API 忽略 prevSummary

**位置**: `manager.go:938-948`

**描述**: Anthropic Compaction API 路径在收到 `prevSummary` 参数后仍直接调用 `Compact()`，不使用 anchored iteration。Compaction API 做完整的服务端压缩，无法传入已有摘要。

**风险**: 无。这是设计决策 — Compaction API 无推理成本且质量可靠，anchored iteration 仅在 local LLM fallback 时生效。与设计文档一致。

**建议**: 无需修改。如未来需要，可在 Compaction fallback 路径中使用 prevSummary。

### F-03 [INFO] ResolvedTriggerPercent 静默截断异常值

**位置**: `config.go:174-178`

**描述**: `CompressionTriggerPercent > 100` 时返回 0 (legacy)，不记录警告日志。用户可能误配 `150` 以为是 "更积极的压缩"，实际退回 legacy 行为。

**风险**: 极低。配置错误不影响功能安全，仅行为不符预期。

**建议**: 可选加 `slog.Warn` 提示异常配置值。当前行为安全（graceful degradation）。

### F-04 [INFO] summarizeWithLocalLLM 无对话长度限制 (pre-existing)

**位置**: `claude_integration.go:268-276`

**描述**: `summarizeWithLocalLLM()` 将全部消息拼接后传入 `SummarizeNewPromptFmt`，无 `maxConversationChars` 限制。而 `manager.go:summarizeMessages()` 有 60K 字符上限。

**风险**: 低。`summarizeWithLocalLLM` 仅在 `CompressWithStrategy` 中调用，该函数在 gateway 层使用，消息量通常有限。LLM provider 自身会截断超长输入。

**注意**: 此为 pre-existing 问题，非本次升级引入。

### F-05 [INFO] Memory extraction 使用原始消息

**位置**: `manager.go:445-452`

**描述**: `CompressIfNeeded` 中异步记忆提取使用 `messages[:len(messages)-keepRecent]`（原始消息），而非 `oldMessages`（masked 消息）。

**风险**: 无。这是正确设计 — 记忆提取需要完整 tool 输出内容才能提取有价值的事实/决策。Masking 仅用于压缩（减少 LLM 输入 token）。

## 安全审查

| 检查项 | 结果 |
|---|---|
| Prompt 注入 | ✅ 用户输入通过 `%s` 格式化注入 prompt，但 system prompt 与 user prompt 分离传入 LLM，无 injection 风险 |
| 信息泄露 | ✅ 遮蔽后的占位符仅包含前 100 字符 preview，不含完整敏感内容 |
| 并发安全 | ✅ `lastSummary` 读用 RLock，写用 Lock；`safeGo` 包装异步操作 |
| 资源泄漏 | ✅ 无新增 goroutine 泄漏点；`safeGo` 已有 panic recovery |
| 配置边界 | ✅ 零值=legacy，负值被处理，百分比超 100 降级为 legacy |

## 正确性审查

| 检查项 | 结果 |
|---|---|
| 向后兼容 | ✅ 所有新字段零值=legacy 行为，DefaultUHMSConfig 不设新字段默认值 |
| Prompt format args | ✅ 所有 `Fmt` 常量的 `%s` 数量与调用处 args 匹配 |
| Prompt 一致性 | ✅ 提取的常量值与原硬编码字符串完全匹配 |
| 编译 | ✅ `go build ./...` 通过 |
| 类型镜像 | ✅ `types_memory.go` 3 字段与 `config.go` 完全对应 |
| configToUHMSConfig | ✅ 3 个新映射遵循已有 `if > 0` 模式 |
| maskObservations | ✅ 创建新切片，不修改原消息；边界条件已修复 (F-01) |
| Anchored iteration | ✅ prevSummary 读/写正确加锁；首次/后续分支逻辑正确 |

## 判定

**PASS** — 1 个 MEDIUM 发现已修复，4 个 INFO 已记录。系统健康可用。
