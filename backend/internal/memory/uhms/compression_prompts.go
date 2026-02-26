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
const StructuredSummaryTemplate = `## Session Intent
[What the user was trying to accomplish in this session]

## Files Modified
[Specific file paths and what changed in each]

## Decisions Made
[Key technical decisions with rationale]

## Errors & Resolutions
[Errors encountered and how they were resolved]

## Current State
[What is working, what is not, build/test status]

## Next Steps
[Explicit pending items and follow-ups]

## Breadcrumbs
[Important identifiers: function names, config keys, error codes, variable names that future context needs]`

// SummarizeSystemPrompt replaces hardcoded "You are a conversation summarizer..." strings.
// Used in: manager.go (summarizeMessages), claude_integration.go (summarizeWithLocalLLM).
const SummarizeSystemPrompt = `You are a structured conversation summarizer for a software development assistant.
You produce summaries using a fixed template with 7 sections: Session Intent, Files Modified, Decisions Made, Errors & Resolutions, Current State, Next Steps, Breadcrumbs.
Preserve ALL technical identifiers: file paths, function names, error codes, config keys.
Never generalize ("various files") — always be specific.
Output the summary using the template sections as markdown headers.`

// SummarizeNewPromptFmt is the user prompt for first-time compression (no existing summary).
// Format args: %s = conversation text, %s = StructuredSummaryTemplate.
const SummarizeNewPromptFmt = `Summarize the following conversation into a structured summary.
Keep all key decisions, code changes, file paths, and important context.
Be specific, not generic. Use the template below.

Conversation:
%s

Use this template for your summary:
%s

Structured Summary:`

// SummarizeAnchoredPromptFmt is the user prompt for anchored iterative compression.
// Merges new conversation content into an existing structured summary instead of regenerating.
// Format args: %s = existing summary, %s = new conversation span.
const SummarizeAnchoredPromptFmt = `You have an existing structured summary and new conversation content.
Merge the new content INTO the existing summary — update each section, ADD new items, do NOT regenerate from scratch.
Preserve all previous breadcrumbs. Remove items only if explicitly superseded.

EXISTING SUMMARY:
%s

NEW CONVERSATION SPAN:
%s

UPDATED STRUCTURED SUMMARY:`

// ObservationMaskPlaceholderFmt is the placeholder for elided tool/system outputs.
// Format arg: %s = first 100 characters of original content.
const ObservationMaskPlaceholderFmt = `[Tool output: %s... (elided)]`

// ArchiveL1PromptFmt is the prompt for generating L1 structured archive summaries.
// Replaces the hardcoded L1 prompt in session_committer.go.
// Format args: %s = conversation text (truncated), %s = StructuredSummaryTemplate.
const ArchiveL1PromptFmt = `Provide a detailed structured summary of this conversation.
Include key decisions, code changes, file paths, and action items.
Use the template below for structure.

Conversation:
%s

Use this template for your summary:
%s

Structured Summary:`

// ---------- Prompt constants for other LLM operations (extracted from hardcoded strings) ----------

// ClassifyCategorySystemPrompt is the system prompt for memory classification.
// Used in: manager.go (classifyCategory).
const ClassifyCategorySystemPrompt = "You are a memory classification system."

// ExtractMemoriesSystemPrompt is the system prompt for memory extraction.
// Used in: manager.go (extractAndStoreMemories).
const ExtractMemoriesSystemPrompt = "You are a memory extraction system. Return valid JSON only."

// MemorySummaryL0SystemPrompt is the system prompt for L0 (abstract) memory summary generation.
// Used in: manager.go (generateMemorySummary).
const MemorySummaryL0SystemPrompt = "You are a concise memory summarizer. Reply with only the abstract."

// MemorySummaryL1SystemPrompt is the system prompt for L1 (overview) memory summary generation.
// Used in: manager.go (generateMemorySummary).
const MemorySummaryL1SystemPrompt = "You are a detailed memory summarizer. Preserve technical details and context."

// ArchiveL0SystemPrompt is the system prompt for L0 archive summary generation.
// Used in: session_committer.go (generateArchiveSummary).
const ArchiveL0SystemPrompt = "You are a concise summarizer. Reply with only the summary."

// ArchiveL1SystemPrompt is the system prompt for L1 archive summary generation.
// Used in: session_committer.go (generateArchiveSummary).
const ArchiveL1SystemPrompt = "You are a detailed summarizer. Preserve technical details."

// CommitExtractSystemPrompt is the system prompt for commit-time memory extraction.
// Used in: session_committer.go (extractMemoriesFromTranscript).
const CommitExtractSystemPrompt = "You are a memory extraction system. Return valid JSON array only, no explanation."
