package uhms

// compression_prompts.go — 集中定义所有压缩/摘要 prompt 常量。
//
// 替换散布在 manager.go、claude_integration.go、session_committer.go 中的硬编码字符串。
// 设计来源:
//   - Factory.ai 36,611 条生产消息评测 (Anchored Iterative Summary)
//   - NeurIPS 2025 JetBrains 论文 (Observation Masking)
//   - Anthropic Context Engineering 文档 (结构化模板)

// StructuredSummaryTemplate is the 7-section template for structured conversation summaries.
// Each section captures a specific aspect of the development session.
const StructuredSummaryTemplate = `## 会话意图
[用户在本次会话中试图完成的任务]

## 修改的文件
[具体的文件路径以及每个文件的更改内容]

## 关键决策
[关键的技术决策及其理由]

## 错误与解决
[遇到的错误以及是如何解决的]

## 当前状态
[目前哪些工作正常，哪些不正常，构建/测试状态]

## 后续步骤
[明确的待办事项和后续跟进项]

## 面包屑线索
[重要的标识符：函数名、配置键、错误代码、未来上下文可能需要的变量名]`

// SummarizeSystemPrompt replaces hardcoded "You are a conversation summarizer..." strings.
// Used in: manager.go (summarizeMessages), claude_integration.go (summarizeWithLocalLLM).
const SummarizeSystemPrompt = `你是一个软件开发助手的结构化对话总结器。
请使用包含7个部分的固定模板生成总结：会话意图、修改的文件、关键决策、错误与解决、当前状态、后续步骤、面包屑线索。
保留所有技术标识符：文件路径、函数名、错误代码、配置键。
永远不要泛泛而谈（例如“各种文件”）——必须具体明确。
使用模板部分作为Markdown标题输出总结。`

// SummarizeNewPromptFmt is the user prompt for first-time compression (no existing summary).
// Format args: %s = conversation text, %s = StructuredSummaryTemplate.
const SummarizeNewPromptFmt = `将以下对话总结为一个结构化的摘要。
保留所有关键决策、代码更改、文件路径和重要上下文。
内容要具体，不要泛泛而谈。请使用以下模板。

对话内容：
%s

请使用此模板进行总结：
%s

结构化总结：`

// SummarizeAnchoredPromptFmt is the user prompt for anchored iterative compression.
// Merges new conversation content into an existing structured summary instead of regenerating.
// Format args: %s = existing summary, %s = new conversation span.
const SummarizeAnchoredPromptFmt = `你拥有一个现有的结构化总结和新的对话内容。
将新内容合并到现有的总结中——更新每个部分，添加新项目，不要从头开始重新生成。
保留所有以前的面包屑线索。只有在明确被取代的情况下才删除项目。

现有总结：
%s

新的对话片段：
%s

更新后的结构化总结：`

// ObservationMaskPlaceholderFmt is the placeholder for elided tool/system outputs.
// Format arg: %s = first 100 characters of original content.
const ObservationMaskPlaceholderFmt = `[工具输出: %s... (已省略)]`

// ArchiveL1PromptFmt is the prompt for generating L1 structured archive summaries.
// Replaces the hardcoded L1 prompt in session_committer.go.
// Format args: %s = conversation text (truncated), %s = StructuredSummaryTemplate.
const ArchiveL1PromptFmt = `提供此对话的详细结构化总结。
包括关键决策、代码更改、文件路径和待办事项。
使用以下模板作为结构。

对话内容：
%s

请使用此模板进行总结：
%s

结构化总结：`

// ---------- Prompt constants for other LLM operations (extracted from hardcoded strings) ----------

// ClassifyCategorySystemPrompt is the system prompt for memory classification.
// Used in: manager.go (classifyCategory).
const ClassifyCategorySystemPrompt = "你是一个记忆分类系统。"

// ExtractMemoriesSystemPrompt is the system prompt for memory extraction.
// Used in: manager.go (extractAndStoreMemories).
const ExtractMemoriesSystemPrompt = "你是一个记忆提取系统。只返回有效的JSON格式。"

// MemorySummaryL0SystemPrompt is the system prompt for L0 (abstract) memory summary generation.
// Used in: manager.go (generateMemorySummary).
const MemorySummaryL0SystemPrompt = "你是一个简明的记忆总结器。只回复摘要内容。"

// MemorySummaryL1SystemPrompt is the system prompt for L1 (overview) memory summary generation.
// Used in: manager.go (generateMemorySummary).
const MemorySummaryL1SystemPrompt = "你是一个详细的记忆总结器。请保留技术细节和上下文。"

// ArchiveL0SystemPrompt is the system prompt for L0 archive summary generation.
// Used in: session_committer.go (generateArchiveSummary).
const ArchiveL0SystemPrompt = "你是一个简明的总结器。只回复总结内容。"

// ArchiveL1SystemPrompt is the system prompt for L1 archive summary generation.
// Used in: session_committer.go (generateArchiveSummary).
const ArchiveL1SystemPrompt = "你是一个详细的总结器。请保留技术细节。"

// CommitExtractSystemPrompt is the system prompt for commit-time memory extraction.
// Used in: session_committer.go (extractMemoriesFromTranscript).
const CommitExtractSystemPrompt = "你是一个记忆提取系统。只返回有效的JSON数组，不要包含解释。"
