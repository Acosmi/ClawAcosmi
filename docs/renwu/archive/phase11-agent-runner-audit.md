# 模块 D: Agent Runner — 重构健康度审计报告

> 审计日期: 2026-02-17
> 方法: `/refactor` 六步循环法 + 隐藏依赖审计
> 验证: `go build` ✅ | `go vet` ✅ | `go test ./internal/agents/...` ✅ (15 包全通过)

---

## 1. 文件映射总表

| TS 源文件 | 行数 | Go 对应 | 行数 | 覆盖率 | 状态 |
|-----------|------|---------|------|--------|------|
| `pi-embedded-runner/run.ts` | 866 | `runner/run.go` | 502 | ~58% | ⚠️ 核心逻辑已覆盖，部分子逻辑缺失 |
| `pi-embedded-runner/run/attempt.ts` | 928 | `runner/attempt_runner.go` | 383 | ~41% | ⚠️ 简化实现，缺大量子逻辑 |
| `pi-embedded-runner/` 其余 23 文件 | 3342 | `runner/run_helpers.go` + 其他 | ~600 | ~18% | ❌ 大量子模块未移植 |
| `model-fallback.ts` | 394 | `models/fallback.go` | 260 | ~66% | ✅ 核心完整 |
| `model-selection.ts` | 447 | `models/selection.go` | 407 | ~91% | ✅ 良好 |
| `model-auth.ts` | 395 | `auth/auth.go` | 344 | ~87% | ✅ 良好 |
| `model-scan.ts` | 513 | `models/config.go` 部分 | ~100 | ~20% | ⚠️ 运行时扫描缺失 |
| `system-prompt.ts` | 648 | `prompt/prompt.go` | 278 | ~43% | ⚠️ 缺 15+ 段落 |
| `bash-tools.exec.ts` | 1630 | `runner/tool_executor.go` | 185 | ~11% | ❌ 严重简化 |
| `bash-tools.process.ts` | 665 | — | 0 | 0% | ❌ 完全缺失 |
| `pi-embedded-subscribe.ts` | 618 | — | 0 | 0% | ❌ 完全缺失 |
| `pi-embedded-utils.ts` | 419 | — | 0 | 0% | ❌ 缺失 |
| `pi-embedded-block-chunker.ts` | 352 | — | 0 | 0% | ❌ 缺失 |
| `pi-embedded-helpers/` (9 文件) | 1395 | `helpers/` (2 文件) | 996 | ~71% | ⚠️ errors.go 覆盖好，其余缺失 |
| `compaction.ts` | 373 | `compaction/compaction.go` | 251 | ~67% | ✅ 核心完整 |
| `tool-policy.ts` | 291 | `scope/tool_policy.go` | 393 | >100% | ✅ 扩展实现 |
| `agent-scope.ts` | 192 | `scope/scope.go` | 305 | >100% | ✅ 扩展实现 |

**总量对比**: TS ~12,077L → Go ~4,505L（~37% 覆盖率）

---

## 2. 隐藏依赖审计 (7 项检查)

| # | 类别 | 结果 | 说明 |
|---|------|------|------|
| 1 | npm 包黑盒行为 | ⚠️ | `@mariozechner/pi-ai` 的 `streamSimple` 流式 API — Go 用 `llmclient/` 替代，但缺流式中间件 |
| 2 | 全局状态/单例 | ⚠️ | TS `runs.ts` 有 `activeRuns` Map + `sessionManagerCache` — Go 无等价全局追踪 |
| 3 | 事件总线/回调链 | ⚠️ | `subscribeEmbeddedPiSession` 整个流式订阅层完全缺失 |
| 4 | 环境变量依赖 | ✅ | `model_resolver_env.go` 已覆盖环境变量解析 |
| 5 | 文件系统约定 | ⚠️ | TS session 文件写锁 (`session-write-lock.ts`) 无 Go 等价 |
| 6 | 协议/消息格式约定 | ✅ | LLM API 请求格式通过 `llmclient/` 正确匹配 |
| 7 | 错误处理约定 | ✅ | `helpers/errors.go` (750L) 覆盖了 TS 错误分类的主要逻辑 |

---

## 3. 关键差异分析

### 3.1 pi-embedded-runner 核心 (P0)

**Go 已实现**: `RunEmbeddedPiAgent` 主循环（auth profile 轮换 + context overflow 重试 + compaction + failover）

**Go 缺失的 TS 子模块**:

- `run/images.ts` (447L) — 历史图片注入到消息，Go 无等价
- `run/payloads.ts` (255L) — 消息 payload 构建，Go 仅 `buildPayloads` 桩
- `compact.ts` (500L) — runner 内 compaction 触发，Go 有 `compaction/` 包但未接入 runner
- `google.ts` (393L) — Google Gemini 特殊处理（grounding, safety settings），Go 无
- `tool-result-truncation.ts` (328L) — 超大工具结果截断，Go 无
- `extensions.ts` (104L) — 扩展配置注入，Go 无
- `extra-params.ts` (156L) — 额外参数处理，Go 无
- `model.ts` (225L) — 模型别名行构建，Go 无

### 3.2 bash-tools (P0)

**TS**: 1630L `bash-tools.exec.ts` + 665L `bash-tools.process.ts` = **2295L 总计**

- PTY 支持 (`pty.js`)、后台进程管理、kill-tree、权限守卫、沙箱集成、输出大小限制、yieldMs 超时

**Go**: 185L `tool_executor.go` — 仅 `exec.Command("bash", "-c", ...)` 基础执行

- ❌ 无 PTY/交互模式
- ❌ 无后台进程管理
- ❌ 无 kill-tree (进程树清理)
- ❌ 无权限守卫 (tool policy 未接入)
- ❌ 无沙箱隔离
- ❌ 高级工具 (browser/message/mcp/cron) 全返回 "not yet implemented"

### 3.3 pi-embedded-subscribe (P1)

**TS**: 618L — 流式订阅层，处理 LLM 流式响应的所有中间逻辑

- 助手文本累积 + 去重 + `<think>`/`<final>` 标签剥离
- 工具结果格式化 + 摘要发射
- compaction 重试协调
- block chunking + reasoning stream
- messaging tool 去重

**Go**: **完全缺失** — `attempt_runner.go` 直接同步调用 `llmclient` 获取完整响应，无流式中间层

### 3.4 system-prompt (P1)

**Go 缺失的段落** (相比 TS 648L → Go 278L):

1. ❌ Tooling 摘要（22 工具名 + 描述）
2. ❌ Tool Call Style 指导
3. ❌ Safety 宪法条款
4. ❌ CLI Quick Reference
5. ❌ Memory Recall 段落
6. ❌ Self-Update 段落
7. ❌ Model Aliases 段落
8. ❌ Sandbox 信息
9. ❌ Reply Tags 段落
10. ❌ Messaging 段落
11. ❌ Voice/TTS 段落
12. ❌ Docs 段落
13. ❌ Silent Replies (HEARTBEAT/SILENT_REPLY_TOKEN)
14. ❌ Heartbeats 段落
15. ❌ Reactions 指导
16. ❌ Context Files (SOUL.md) 注入
17. ❌ Reasoning Format 提示

### 3.5 model-fallback / selection / auth (P2 — 低风险)

✅ 这三个模块覆盖良好：

- `fallback.go`: `ResolveFallbackCandidates` + `RunWithModelFallback` + auth cooldown skip
- `selection.go`: `ParseModelRef` + `BuildAllowedModelSet` + `ResolveThinkingDefault`
- `auth.go`: `AuthStore` + cooldown 指数退避 + `MarkProfileFailure/Used/Good`

---

## 4. 分级行动项

### P0 — 阻塞级 (影响基本 agent 执行)

| ID | 项目 | 影响 | 工作量 |
|----|------|------|--------|
| D-P0-1 | `tool_executor.go` 补全高级工具 | agent 无法使用除 bash/read/write/list 外的工具 | 大 |
| D-P0-2 | bash PTY + 后台进程 + kill-tree | 交互命令/长时任务不可用 | 大 |
| D-P0-3 | tool-result-truncation 移植 | 大工具输出导致 context overflow | 中 |

### P1 — 重要 (影响功能完整度)

| ID | 项目 | 影响 | 工作量 |
|----|------|------|--------|
| D-P1-1 | `system-prompt.ts` 缺失段落补全 | agent 行为缺乏关键指导 | 中 |
| D-P1-2 | `pi-embedded-subscribe` 流式层 | 无流式输出、无实时反馈 | 大 |
| D-P1-3 | `run/images.ts` 图片注入 | 多模态对话不可用 | 中 |
| D-P1-4 | `google.ts` Gemini 特殊处理 | Gemini 模型功能受限 | 中 |
| D-P1-5 | 全局 activeRuns 追踪 | 无法防止并发 run 冲突 | 小 |

### P2 — 增强 (可延迟)

| ID | 项目 | 影响 | 工作量 |
|----|------|------|--------|
| D-P2-1 | `model-scan.ts` 运行时扫描 | 无动态模型发现 | 中 |
| D-P2-2 | session-write-lock 移植 | 并发写入风险 | 小 |
| D-P2-3 | `bash-tools.process.ts` 进程管理 | 后台进程管理 | 中 |
| D-P2-4 | `pi-embedded-utils.ts` 工具规范化 | 消息规范化缺失 | 小 |
| D-P2-5 | `pi-embedded-block-chunker.ts` 块分割 | 分块输出 | 小 |

---

## 5. 验证结果

```
go build ./...    ✅ 通过
go vet ./...      ✅ 通过
go test ./internal/agents/...  ✅ 15 包全通过 (总耗时 ~2.4s)
```

---

## 6. 结论

Agent Runner 模块的 **模型层** (selection/fallback/auth) 移植质量良好（~80%+覆盖率）。

**执行层** (runner/tools/subscribe) 存在显著差距：

- 核心 `RunEmbeddedPiAgent` 主循环已实现，但缺少大量子逻辑
- 工具执行极度简化（2295L → 185L）
- 流式订阅层完全缺失
- 系统提示词缺 17 个段落

建议优先级：D-P0-1/2/3 → D-P1-1/2 → 其余按需。
