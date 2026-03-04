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
// Bug#11: 当 LLM 不可用时，降级到关键词启发式提取。
func extractMemoriesFromTranscript(ctx context.Context, m *DefaultManager, text string) []extractedMemory {
	if m.llm == nil {
		return extractMemoriesHeuristic(text)
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
		provider, model := m.LLMInfo()
		slog.Warn("uhms/commit: extraction LLM failed, fallback to heuristic",
			"error", err, "provider", provider, "model", model)
		return extractMemoriesHeuristic(text)
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

// ---------- Bug#11: 启发式记忆提取（LLM 不可用时降级） ----------

const maxHeuristicExtract = 5

// heuristicKeywords 按类别组织关键词，用于启发式分类。
var heuristicKeywords = map[MemoryCategory][]string{
	CatPreference: {
		"喜欢", "偏好", "偏爱", "更喜欢", "最爱", "讨厌", "不喜欢", "习惯用", "总是用",
		"prefer", "like", "love", "hate", "dislike", "always use", "favorite",
	},
	CatProfile: {
		"我是", "我的名字", "我叫", "我在", "我住", "我的职业", "我的工作", "我做",
		"my name", "i am a", "i work", "i live", "my job", "my role",
	},
	CatGoal: {
		"目标", "计划", "打算", "想要", "准备", "希望", "需要完成", "打算做",
		"goal", "plan to", "want to", "aim to", "intend to", "hope to",
	},
	CatFact: {
		"记住", "注意", "重要的是", "关键是", "需要记住", "别忘了", "一定要",
		"remember", "important", "key point", "note that", "keep in mind",
	},
	CatSkill: {
		"我会", "我擅长", "我熟悉", "我懂", "我学过", "我的技能", "我精通",
		"i know", "i can", "skilled in", "proficient", "experienced with", "familiar with",
	},
}

// extractMemoriesHeuristic 基于关键词的启发式记忆提取（LLM 不可用时的降级方案）。
// 只处理 [user]: 前缀行，按关键词分类，评分阈值 ≥2.0 才提取，Jaccard 去重。
func extractMemoriesHeuristic(text string) []extractedMemory {
	if len(text) < 20 {
		return nil
	}

	lines := strings.Split(text, "\n")
	var candidates []extractedMemory
	seen := make([]string, 0) // 已提取内容，用于去重

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 只处理用户消息
		if !strings.HasPrefix(line, "[user]:") {
			continue
		}
		content := strings.TrimSpace(strings.TrimPrefix(line, "[user]:"))
		if len(content) < 8 {
			continue
		}

		cat, score := classifyByKeywords(content)
		if score < 2.0 {
			continue
		}

		// Jaccard 去重
		if isDuplicateHeuristic(content, seen) {
			continue
		}

		candidates = append(candidates, extractedMemory{
			Content:  content,
			MemType:  classifyCategoryToType(cat),
			Category: cat,
		})
		seen = append(seen, content)

		if len(candidates) >= maxHeuristicExtract {
			break
		}
	}

	return candidates
}

// classifyByKeywords 按关键词匹配返回最佳类别和评分。
// 每匹配一个关键词 +2.0 分。
func classifyByKeywords(text string) (MemoryCategory, float64) {
	lower := strings.ToLower(text)
	bestCat := CatFact
	bestScore := 0.0

	for cat, keywords := range heuristicKeywords {
		score := 0.0
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				score += 2.0
			}
		}
		if score > bestScore {
			bestScore = score
			bestCat = cat
		}
	}

	return bestCat, bestScore
}

// classifyCategoryToType 将 category 映射到 MemoryType。
func classifyCategoryToType(cat MemoryCategory) MemoryType {
	switch cat {
	case CatPreference, CatHabit:
		return MemTypeSemantic
	case CatProfile:
		return MemTypePermanent
	case CatGoal, CatTask:
		return MemTypeEpisodic
	case CatSkill:
		return MemTypeProcedural
	default:
		return MemTypeSemantic
	}
}

// isDuplicateHeuristic 使用 Jaccard 相似度和子集包含检查重复。
// 使用 mixedTokenSet 同时支持空格分词（英文）和 bigram（中文）。
func isDuplicateHeuristic(content string, seen []string) bool {
	newSet := mixedTokenSet(content)
	if len(newSet) == 0 {
		return false
	}
	for _, s := range seen {
		oldSet := mixedTokenSet(s)
		// Jaccard 相似度检查
		if jaccardSimilarity(newSet, oldSet) >= 0.5 {
			return true
		}
		// 子集包含检查: 短文本的 token 全部出现在长文本中
		if isSubset(newSet, oldSet) || isSubset(oldSet, newSet) {
			return true
		}
	}
	return false
}

// isSubset 检查 a 是否为 b 的子集（a 的所有 token 都在 b 中）。
func isSubset(a, b map[string]bool) bool {
	if len(a) == 0 || len(a) > len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// mixedTokenSet 生成混合 token 集合：英文按空格分词，中文按字符 bigram。
// 例: "我喜欢 dark mode" → {"我喜", "喜欢", "dark", "mode"}
func mixedTokenSet(text string) map[string]bool {
	set := make(map[string]bool)
	lower := strings.ToLower(text)

	// 英文空格分词: 只保留含 ASCII 字母的 token（排除纯 CJK 字符串被当作单词）
	for _, w := range strings.Fields(lower) {
		if len(w) > 2 && containsLatin(w) {
			set[w] = true
		}
	}

	// 中文字符 bigram
	runes := []rune(lower)
	for i := 0; i+1 < len(runes); i++ {
		if isCJK(runes[i]) && isCJK(runes[i+1]) {
			set[string(runes[i:i+2])] = true
		}
	}

	return set
}

// containsLatin 检查字符串是否包含至少一个 ASCII 字母。
func containsLatin(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

// isCJK 判断是否为 CJK 统一汉字（U+4E00–U+9FFF）。
func isCJK(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}

// jaccardSimilarity 计算两个词集的 Jaccard 相似度。
func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	intersection := 0
	for w := range a {
		if b[w] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
