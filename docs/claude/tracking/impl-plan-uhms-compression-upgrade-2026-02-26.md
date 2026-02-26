---
document_type: Tracking
status: In Progress
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-uhms-compression-upgrade.md
skill5_verified: true
---

# UHMS 上下文压缩技能 + 压缩管线升级

## 目标

用专业的压缩技能替换硬编码 prompt，实现 Anchored Iterative + 结构化模板 + Observation Masking 三大升级。

## 实施顺序 & 进度

- [x] **1. NEW: `backend/internal/memory/uhms/compression_prompts.go`** — 结构化 prompt 常量
- [x] **2. MODIFY: `backend/internal/memory/uhms/config.go`** — 加 3 字段 + 访问器
- [x] **3. MODIFY: `backend/pkg/types/types_memory.go`** — 镜像 3 字段
- [x] **4. MODIFY: `backend/internal/gateway/server.go`** — configToUHMSConfig 映射
- [x] **5. MODIFY: `backend/internal/memory/uhms/manager.go`** — 核心: lastSummary + maskObservations + 重写 CompressIfNeeded/summarizeMessages
- [x] **6. MODIFY: `backend/internal/memory/uhms/claude_integration.go`** — 同步 prompt
- [x] **7. MODIFY: `backend/internal/memory/uhms/session_committer.go`** — L1 结构化归档
- [x] **8. ~~`docs/skills/tools/context-compressor/SKILL.md`~~** — 已删除: 压缩是 UHMS 后端自动执行，主智能体加载此技能浪费 tokens。架构文档见 `docs/claude/goujia/arch-uhms-memory-system.md`
- [x] **9. 编译验证** — `cd backend && go build ./...` ✅ 通过

## 已读取的文件 (全部已读完)

| 文件 | 行数 | 关键发现 |
|---|---|---|
| `config.go` | 177 | UHMSConfig 结构体在 L64-103，DefaultUHMSConfig() 在 L106-121，已有访问器模式 |
| `types_memory.go` | 92 | MemoryUHMSConfig 在 L77-92，已有 LLMProvider/LLMModel/LLMBaseURL 3 个字段 |
| `manager.go` | 1070 | DefaultManager L39-55; CompressIfNeeded L330-415; summarizeMessages L859-903; classifyCategory L659-683; extractAndStoreMemories L905-935; generateMemorySummary L749-803 |
| `claude_integration.go` | 296 | summarizeWithLocalLLM L268-286 (hardcoded prompt); CompressWithStrategy L226-265 |
| `session_committer.go` | 220 | generateArchiveSummary L79-124 (L1 prompt at L106-112); extractMemoriesFromTranscript L127-161 |
| `server.go` | configToUHMSConfig at L358-389; 需在 L387 后加 3 个字段映射 |

## 修改细节

### 1. compression_prompts.go (新建)

6 个常量:
- `StructuredSummaryTemplate` — 7 段模板 (Session Intent / Files Modified / Decisions Made / Errors & Resolutions / Current State / Next Steps / Breadcrumbs)
- `SummarizeSystemPrompt` — 替换 "You are a conversation summarizer..." (manager.go:902, claude_integration.go:284)
- `SummarizeNewPromptFmt` — 首次压缩 user prompt (manager.go:893-900)
- `SummarizeAnchoredPromptFmt` — 增量合并 user prompt (新增)
- `ObservationMaskPlaceholderFmt` — 被遮蔽 tool 输出占位符 (新增)
- `ArchiveL1PromptFmt` — 归档 L1 结构化摘要 (session_committer.go:106-112)

### 2. config.go

UHMSConfig 加 3 字段:
```go
CompressionTriggerPercent int `json:"compressionTriggerPercent,omitempty"` // 0=legacy
ObservationMaskTurns      int `json:"observationMaskTurns,omitempty"`      // 0=关闭
KeepRecentMessages        int `json:"keepRecentMessages,omitempty"`        // 0=默认5
```
DefaultUHMSConfig() 中不设默认值 (零值=legacy)。
加 `ResolvedKeepRecent() int` 和 `ResolvedTriggerPercent() int` 访问器。

### 3. types_memory.go

MemoryUHMSConfig 镜像加同样 3 字段。

### 4. server.go

configToUHMSConfig() 在 L387 后加:
```go
if c.CompressionTriggerPercent > 0 { cfg.CompressionTriggerPercent = c.CompressionTriggerPercent }
if c.ObservationMaskTurns > 0 { cfg.ObservationMaskTurns = c.ObservationMaskTurns }
if c.KeepRecentMessages > 0 { cfg.KeepRecentMessages = c.KeepRecentMessages }
```

### 5. manager.go 核心改动

- 5a. DefaultManager 加 `lastSummary string` 字段 (受 `mu` 保护)
- 5b. 新增 `maskObservations(messages []Message) []Message` 方法
  - 从末尾倒数 N 个 user turn，之前的 tool/system 消息内容替换为 `[Tool output: {前100字符}... (elided)]`
  - ObservationMaskTurns==0 直接返回原切片
- 5c. 重写 CompressIfNeeded():
  - 百分比触发: triggerPercent>0 ? totalTokens > budget*percent/100 : totalTokens >= budget
  - maskObservations() 遮蔽旧 tool 输出
  - 保留 cfg.ResolvedKeepRecent() 条
  - summarizeMessages() 使用 anchored iteration
  - 存储 lastSummary (加锁写入)
- 5d. 重写 summarizeMessages():
  - Anthropic Compaction 仍优先
  - lastSummary != "" → SummarizeAnchoredPromptFmt 增量合并
  - else → SummarizeNewPromptFmt + StructuredSummaryTemplate
  - system prompt 改用 SummarizeSystemPrompt
- 5e. classifyCategory / extractAndStoreMemories / generateMemorySummary 的 system prompt 提取为常量

### 6. claude_integration.go

summarizeWithLocalLLM() 改用 SummarizeSystemPrompt + SummarizeNewPromptFmt

### 7. session_committer.go

generateArchiveSummary() L1 prompt 改用 ArchiveL1PromptFmt + StructuredSummaryTemplate

### 8. SKILL.md

`docs/skills/tools/context-compressor/SKILL.md` — agent 可读技能文件

## 关键设计决策

- prompt 存放: Go 常量文件 (编译检查)
- lastSummary: 内存字段不持久化 (单次会话)
- 新配置默认值: 全部零值=legacy，现有部署零破坏
- Anthropic Compaction: 保持第一优先 (无推理成本)
- Observation Masking: 仅 tool/system role (NeurIPS 2025: tool 输出占 84% tokens)
