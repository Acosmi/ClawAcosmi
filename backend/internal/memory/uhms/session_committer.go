package uhms

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// commitSession extracts memories from a conversation transcript and stores them.
//
// Pipeline:
//  1. Archive: LLM 摘要对话 → VFS 归档 (L0/L1/L2 full transcript)
//  2. Extract: LLM 提取 6 类记忆 (preference/fact/skill/event/goal/insight)
//  3. Dedupe + Store: 去重 → 写入 SQLite + VFS
func commitSession(ctx context.Context, m *DefaultManager, userID, sessionKey string, transcript []Message) (*CommitResult, error) {
	if len(transcript) == 0 {
		return &CommitResult{SessionKey: sessionKey}, nil
	}

	result := &CommitResult{SessionKey: sessionKey}

	// 构建对话文本
	conversationText := buildTranscriptText(transcript)

	// Step 1: 归档摘要 → VFS (L0/L1 summary + L2 full transcript)
	l0Summary, l1Overview := generateArchiveSummary(ctx, m, conversationText)
	if l0Summary != "" {
		archivePath, err := m.vfs.WriteArchive(userID, sessionKey, l0Summary, l1Overview, conversationText)
		if err != nil {
			slog.Warn("uhms/commit: archive write failed", "error", err)
		} else {
			result.ArchivePath = archivePath
		}
	}

	// Step 2: 提取记忆
	extracted := extractMemoriesFromTranscript(ctx, m, conversationText)
	if len(extracted) == 0 {
		slog.Debug("uhms/commit: no memories extracted", "sessionKey", sessionKey)
		return result, nil
	}

	// Step 3: 去重 + 存储
	for _, em := range extracted {
		mem, err := m.AddMemory(ctx, userID, em.Content, em.MemType, em.Category)
		if err != nil {
			slog.Debug("uhms/commit: add memory failed", "error", err)
			continue
		}
		result.MemoryIDs = append(result.MemoryIDs, mem.ID)
		result.MemoriesCreated++
	}

	// 估算节省的 tokens
	originalTokens := m.estimateTokens(conversationText)
	summaryTokens := m.estimateTokens(l0Summary + l1Overview)
	result.TokensSaved = originalTokens - summaryTokens

	slog.Info("uhms/commit: session committed",
		"sessionKey", sessionKey,
		"memoriesCreated", result.MemoriesCreated,
		"tokensSaved", result.TokensSaved,
	)
	return result, nil
}

// buildTranscriptText converts messages to a readable text format.
func buildTranscriptText(messages []Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n\n", msg.Role, msg.Content))
	}
	return sb.String()
}

// generateArchiveSummary generates L0 (1-2 sentence) and L1 (paragraph) summaries.
func generateArchiveSummary(ctx context.Context, m *DefaultManager, text string) (l0, l1 string) {
	if m.llm == nil {
		// 无 LLM: 简单截取
		l0 = truncate(text, 200)
		l1 = truncate(text, 2000)
		return
	}

	// L0: 极短摘要
	l0Prompt := fmt.Sprintf(`Summarize this conversation in 1-2 sentences (under 100 words).
Focus on the main topic and outcome.

Conversation:
%s

Summary:`, truncate(text, 6000))

	l0Result, err := m.llm.Complete(ctx,
		ArchiveL0SystemPrompt,
		l0Prompt)
	if err != nil {
		l0 = truncate(text, 200)
	} else {
		l0 = strings.TrimSpace(l0Result)
	}

	// L1: 结构化概述
	l1Prompt := fmt.Sprintf(ArchiveL1PromptFmt, truncate(text, 10000), StructuredSummaryTemplate)

	l1Result, err := m.llm.Complete(ctx,
		ArchiveL1SystemPrompt,
		l1Prompt)
	if err != nil {
		l1 = truncate(text, 2000)
	} else {
		l1 = strings.TrimSpace(l1Result)
	}

	return
}

// extractMemoriesFromTranscript uses LLM to extract structured memories.
func extractMemoriesFromTranscript(ctx context.Context, m *DefaultManager, text string) []extractedMemory {
	if m.llm == nil {
		return nil
	}

	prompt := fmt.Sprintf(`Analyze this conversation and extract important memories.
For each memory, provide:
- "content": the specific fact, preference, or decision (1-3 sentences)
- "type": one of [episodic, semantic, procedural, permanent]
- "category": one of [preference, habit, profile, skill, relationship, event, opinion, fact, goal, task, reminder, insight, summary]

Focus on:
1. User preferences and habits
2. Important decisions made
3. Technical knowledge shared
4. Tasks and goals mentioned
5. Key facts and insights

Return as a JSON array. If nothing notable, return [].

Conversation:
%s

JSON:`, truncate(text, 8000))

	result, err := m.llm.Complete(ctx,
		CommitExtractSystemPrompt,
		prompt)
	if err != nil {
		slog.Debug("uhms/commit: extraction LLM failed", "error", err)
		return nil
	}

	return parseExtractedJSON(result)
}

// parseExtractedJSON parses the LLM output into structured memories.
func parseExtractedJSON(raw string) []extractedMemory {
	raw = strings.TrimSpace(raw)

	// 找 JSON 数组边界
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end <= start {
		return nil
	}
	raw = raw[start : end+1]

	type rawEntry struct {
		Content  string `json:"content"`
		Type     string `json:"type"`
		Category string `json:"category"`
	}

	var entries []rawEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		slog.Debug("uhms/commit: JSON parse failed", "error", err, "raw", truncate(raw, 200))
		return nil
	}

	memories := make([]extractedMemory, 0, len(entries))
	for _, e := range entries {
		if e.Content == "" {
			continue
		}
		memories = append(memories, extractedMemory{
			Content:  e.Content,
			MemType:  normalizeMemoryType(e.Type),
			Category: normalizeCategory(e.Category),
		})
	}
	return memories
}

func normalizeMemoryType(t string) MemoryType {
	t = strings.ToLower(strings.TrimSpace(t))
	for _, mt := range AllMemoryTypes {
		if string(mt) == t {
			return mt
		}
	}
	return MemTypeEpisodic
}

func normalizeCategory(c string) MemoryCategory {
	c = strings.ToLower(strings.TrimSpace(c))
	for _, cat := range AllCategories {
		if string(cat) == c {
			return cat
		}
	}
	return CatFact
}
