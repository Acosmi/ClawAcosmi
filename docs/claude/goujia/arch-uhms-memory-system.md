---
document_type: Architecture
status: Current
created: 2026-02-26
last_updated: 2026-02-26
---

# UHMS 记忆系统架构

> Unified Hierarchical Memory System — `backend/internal/memory/uhms/`

## 系统概览

UHMS 是 OpenAcosmi 的嵌入式记忆系统，提供:
- **上下文压缩**: 长会话自动摘要，保持在 token 预算内
- **记忆持久化**: 从对话中提取事实/偏好/决策，跨会话保留
- **层级存储**: L0 (极短摘要) → L1 (段落概述) → L2 (完整内容)
- **向量检索**: 可选的多后端向量搜索 (builtin/FFI/Qdrant/hybrid)

## 文件清单

| 文件 | 职责 | LOC |
|---|---|---|
| `config.go` | UHMSConfig 结构体 + 默认值 + 访问器 | ~195 |
| `manager.go` | DefaultManager — 压缩/存储/检索/VFS 编排 | ~1100 |
| `compression_prompts.go` | 13 个 prompt 常量 (集中管理) | ~110 |
| `claude_integration.go` | Anthropic Compaction API + Provider 感知策略 | ~296 |
| `session_committer.go` | 会话提交管线: 归档 + 记忆提取 | ~220 |
| `store.go` | SQLite + FTS5 持久化 | — |
| `vfs.go` | L0/L1/L2 文件系统层级存储 | — |
| `cache.go` | LRU 内存缓存 | — |
| `dedup.go` | FTS5 去重 | — |
| `decay.go` | FSRS-6 记忆衰减 | — |
| `interfaces.go` | LLMProvider / VectorIndex / EmbeddingProvider 接口 | — |
| `llm_adapter.go` | LLMClientAdapter (接入各 LLM provider) | — |
| `types.go` | Memory / Message / CommitResult 等类型 | — |

## 压缩管线架构

### 数据流

```
消息流入
    ↓
触发判定 (百分比/legacy)
    ↓ (超阈值)
Observation Masking (遮蔽旧 tool 输出)
    ↓
分割: 旧消息 | 保留最近 N 条
    ↓
摘要生成 ─────────────────────┐
  ├─ Anthropic Compaction (优先) │
  ├─ Anchored Iterative (增量)   │ → lastSummary 存储
  └─ New Structured (首次)       │
    ↓                           │
记忆提取 (异步 safeGo)          │
    ↓                           │
记忆注入 (相关记忆检索)          │
    ↓                           │
压缩输出 [记忆块] + [摘要] + [最近消息]
```

### 三大升级技术

#### 1. Anchored Iterative Summarization

- **问题**: 每次压缩全量重生成摘要，丢失早期上下文
- **方案**: `lastSummary` 字段 (内存，受 `mu` 保护) 存储上次结构化摘要
- **首次**: `SummarizeNewPromptFmt` + `StructuredSummaryTemplate`
- **后续**: `SummarizeAnchoredPromptFmt` — 增量合并新内容到已有摘要
- **效果**: 质量评分 3.70 vs 3.44 (+8%)，来源: Factory.ai 36,611 条生产消息评测

#### 2. 结构化 7 段模板

```
## Session Intent      — 用户本次会话的目标
## Files Modified      — 具体文件路径和变更
## Decisions Made      — 关键技术决策和理由
## Errors & Resolutions — 遇到的错误和解决方式
## Current State       — 当前状态 (构建/测试/功能)
## Next Steps          — 待办事项和后续步骤
## Breadcrumbs         — 重要标识符 (函数名/配置键/错误码)
```

- **问题**: 自由文本摘要丢失结构化信息 ("various files" 等模糊描述)
- **效果**: 文件追踪评分 2.45 vs 2.19，来源: Factory.ai 生产模板

#### 3. Observation Masking

- **问题**: tool 输出占 84% trajectory tokens，大量冗余
- **方案**: `maskObservations()` 方法 — 从末尾倒数 N 个 user turn，之前的 tool/system 消息替换为:
  ```
  [Tool output: {前100字符}... (elided)]
  ```
- **配置**: `ObservationMaskTurns` (0=关闭)
- **效果**: 减 60-80% tokens，零质量损失，来源: NeurIPS 2025 JetBrains 论文

### 触发机制

| 模式 | 条件 | 配置 |
|---|---|---|
| 百分比模式 | `totalTokens > budget * percent / 100` | `CompressionTriggerPercent: 75` (默认) |
| Legacy 模式 | `totalTokens >= budget` | `CompressionTriggerPercent: 0` |

### 摘要策略优先级

1. **Anthropic Compaction API** — 服务端压缩，零推理成本，POST `/v1/messages/compaction` (Beta)
2. **Local LLM + 结构化 prompt** — Compaction 失败或非 Anthropic provider 时
3. **Simple truncation** — 无 LLM 时，按 200 字截断

## 配置字段

### 核心配置 (UHMSConfig / MemoryUHMSConfig)

| 字段 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `enabled` | bool | false | 启用 UHMS |
| `dbPath` | string | `~/.openacosmi/memory/uhms.db` | SQLite 路径 |
| `vfsPath` | string | `~/.openacosmi/memory/vfs/` | VFS 根目录 |
| `vectorMode` | string | "off" | off/builtin/ffi/segment/qdrant/hybrid |
| `compressionThreshold` | int | 200000 | Token 预算 |
| `compressionTriggerPercent` | int | 75 | 压缩触发百分比 (0=legacy, 参考 ACON 70%) |
| `observationMaskTurns` | int | 3 | 保留完整 tool 输出的最近 user turn 数 |
| `keepRecentMessages` | int | 5 | 压缩时保留的最近消息数 |
| `decayEnabled` | *bool | true | FSRS-6 记忆衰减 |
| `decayIntervalHours` | int | 6 | 衰减周期 (小时) |
| `maxMemories` | int | 100000 | 每用户最大记忆数 |
| `tieredLoadingEnabled` | *bool | true | L0/L1/L2 渐进加载 |
| `embeddingProvider` | string | "auto" | 嵌入模型 provider |
| `qdrantEndpoint` | string | `http://localhost:6334` | Qdrant 服务器地址 |
| `llmProvider` | string | "" | 空=跟随 agent |
| `llmModel` | string | "" | 空=按 provider 默认 |
| `llmBaseUrl` | string | "" | 空=使用 provider 默认 URL |

### 向后兼容

所有新增字段 (`compressionTriggerPercent`, `observationMaskTurns`, `keepRecentMessages`) 零值 = legacy 行为。现有部署无需修改配置。

## Prompt 常量 (compression_prompts.go)

集中管理所有 LLM prompt，替换分散在 3 个文件中的 5+ 个硬编码字符串:

| 常量 | 用途 | 引用位置 |
|---|---|---|
| `StructuredSummaryTemplate` | 7 段摘要模板 | manager.go, session_committer.go |
| `SummarizeSystemPrompt` | 压缩 system prompt | manager.go, claude_integration.go |
| `SummarizeNewPromptFmt` | 首次压缩 user prompt | manager.go, claude_integration.go |
| `SummarizeAnchoredPromptFmt` | 增量合并 user prompt | manager.go |
| `ObservationMaskPlaceholderFmt` | 遮蔽占位符 | manager.go |
| `ArchiveL1PromptFmt` | 归档 L1 结构化摘要 | session_committer.go |
| `ClassifyCategorySystemPrompt` | 记忆分类 | manager.go |
| `ExtractMemoriesSystemPrompt` | 记忆提取 | manager.go |
| `MemorySummaryL0SystemPrompt` | L0 摘要生成 | manager.go |
| `MemorySummaryL1SystemPrompt` | L1 摘要生成 | manager.go |
| `ArchiveL0SystemPrompt` | 归档 L0 | session_committer.go |
| `ArchiveL1SystemPrompt` | 归档 L1 | session_committer.go |
| `CommitExtractSystemPrompt` | 提交记忆提取 | session_committer.go |

## 会话提交管线

```
commitSession()
  ├─ buildTranscriptText()         — 消息 → 纯文本
  ├─ generateArchiveSummary()      — L0 (1-2句) + L1 (结构化7段)
  │   ├─ L0: ArchiveL0SystemPrompt
  │   └─ L1: ArchiveL1PromptFmt + StructuredSummaryTemplate
  ├─ WriteArchive(l0, l1, l2)     — L2 全文归档 (截断 200KB)
  ├─ extractMemoriesFromTranscript() — JSON 数组提取
  │   └─ 6 类: preference/fact/skill/event/goal/insight
  │   └─ 4 类型: episodic/semantic/procedural/permanent
  └─ AddMemory() × N              — 去重 + SQLite + VFS 存储
```

### L2 全文归档

`WriteArchive()` 同时持久化 L0/L1/L2 三层:
- L0: 1-2 句摘要 (~100 tokens)
- L1: 结构化 7 段概述 (~2K tokens)
- L2: 原始对话全文 (截断 200KB ≈ 50K tokens)

跨会话恢复时可通过 `ReadByVFSPath(path, 2)` 获取 L2 原文。

### 压缩→VFS 桥

`CompressIfNeeded()` 生成 summary 后同步写入 VFS 存档:
- 异步写入 `archives/compress_{timestamp}/` (L0 截取/L1 摘要/L2 被压缩消息原文)
- 异步入库 `semantic/summary` 类型记忆 (可被 BuildContextBlock 渐进检索)

### 子智能体上下文传播

`BuildContextBrief()` 从 `lastSummary` 提取 L0 级别简报 (~200 tokens):
- 提取 Session Intent + Files Modified + Current State 三段
- 通过 `_context_brief` 字段注入 coder/argus 工具调用参数
- 减少 inter-agent misalignment (参考: 36.9% 多智能体失败归因于此)

### Rust 精确 Token 计数

`countTokensBPE()` 通过 FFI 调用 tiktoken-rs cl100k_base:
- CGO 模式: Rust FFI 精确 BPE O(n)，比 rune 估算准确 40%+
- 纯 Go fallback: `countTokensRune()` (±40% 误差)
- 源码: `openviking-ffi/src/tokenizer_api.rs` + `uhms/tokenizer_cgo.go`

## Provider 感知压缩 (claude_integration.go)

```
SelectCompressionStrategy(provider, hasLLM)
  ├─ "anthropic" → StrategyAnthropicCompaction
  ├─ hasLLM     → StrategyLocalLLM
  └─ else       → StrategyTruncate

CompressWithStrategy(strategy, ...)
  ├─ AnthropicCompaction → Compact() API → fallback LocalLLM → fallback Truncate
  ├─ LocalLLM → summarizeWithLocalLLM() → SummarizeSystemPrompt + SummarizeNewPromptFmt
  └─ Truncate → truncateMessages()
```

- **Compaction API**: POST `/v1/messages/compaction`, Beta header `prompt-caching-2024-07-31`
- **Prompt Caching**: `BuildCacheableSystemBlocks()` 预备 (ephemeral cache_control)，需 llmclient 扩展支持

## Gateway 集成

| 组件 | 位置 | 说明 |
|---|---|---|
| `configToUHMSConfig()` | server.go | types.MemoryUHMSConfig → uhms.UHMSConfig |
| `buildUHMSLLMAdapter()` | server.go | UHMS 独立 LLM > agent provider 自动选择 |
| `boot.go` | gateway/ | 初始化 Manager + CompactionClient + uhmsBridgeAdapter |
| `uhmsBridgeAdapter` | server.go | EmbeddedAttemptRunner.UHMSBridge 注入 |

### LLM 热替换

- RPC `memory.uhms.llm.set`: 更新 provider/model/baseUrl → `SetLLMProvider()` + CompactionClient 联动
- RPC `memory.uhms.llm.get`: 读取当前 LLM 配置

## 并发安全

- `DefaultManager.mu` (RWMutex): 保护 `closed`, `lastSummary`
- `safeGo()`: 所有 fire-and-forget goroutine 包装，panic recovery + stack trace
- 记忆提取异步执行 (`safeGo("memory_extraction", ...)`)，不阻塞压缩流程

## 技能文件

- `docs/skills/tools/context-compressor/SKILL.md` — Agent 可读技能，解释压缩策略和长会话最佳实践

## 关键设计决策

| 决策 | 选择 | 原因 |
|---|---|---|
| prompt 存放 | Go 常量文件 (`compression_prompts.go`) | 类型安全、编译检查、IDE 补全 |
| lastSummary 存储 | 内存字段 + VFS 持久化 | `.last_summary` 文件，gateway 重启恢复 |
| 新配置默认值 | 75% 触发 + 3 turn mask | 参考 ACON 最佳实践 (70% 触发) |
| Anthropic Compaction | 第一优先 | 服务端压缩无推理成本 |
| Observation Masking | 仅 tool/system role | tool 输出占 84% tokens (NeurIPS 2025) |
| safeGo 包装 | CockroachDB Stopper 模式 | 禁止裸 `go`，统一 panic recovery |
| 结构化模板 | 强制 7 段 section | 消除模糊描述，保留技术标识符 |

## 相关文档

- **追踪**: `docs/claude/tracking/impl-plan-uhms-compression-upgrade-2026-02-26.md` (9/9 完成)
- **审计**: `docs/claude/audit/audit-2026-02-26-uhms-p3-runner-gateway.md` (PASS)
- **延迟项**: `docs/claude/deferred/uhms-p3-deferred.md` (D-04/D-05 待做)
- **集成追踪**: `docs/claude/tracking/impl-plan-uhms-integration-2026-02-26.md`
- **LLM 配置追踪**: `docs/claude/tracking/impl-plan-uhms-llm-config-2026-02-26.md`
- **L0/L1/L2 补全**: `docs/claude/tracking/impl-plan-l0l1l2-compression-fix-2026-02-26.md`
