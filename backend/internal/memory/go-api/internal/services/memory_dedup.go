// Package services — Extract → Dedupe → Merge 事实管线。
// 参考 mem0 两阶段更新机制，在记忆写入前先提取事实并去重。
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// Fact 表示从内容中提取的结构化事实。
type Fact struct {
	Content    string  `json:"fact"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
}

// DedupPair 表示一对需要合并的事实（新 + 旧）。
type DedupPair struct {
	NewFact      Fact   `json:"new_fact"`
	ExistingFact Fact   `json:"existing_fact"`
	ExistingID   string `json:"existing_id"` // 数据库中已有记忆的 ID
}

// FactPipelineResult 表示事实管线的处理结果。
type FactPipelineResult struct {
	NewFacts     []Fact      // 全新事实，需要插入
	UpdatePairs  []DedupPair // 需要合并更新的事实对
	SkippedCount int         // 完全重复被跳过的数量
}

// --- Prompts ---

const extractFactsPrompt = `Extract distinct factual statements from the following text.
Return a JSON array of objects with keys: "fact", "category", "confidence".
Categories: preference, habit, profile, skill, relationship, event, opinion, fact, goal, task, reminder.
Confidence: 0.0 to 1.0 (how certain this fact is).

Text: "%s"

Respond with ONLY a JSON array, no explanation. Example:
[{"fact": "User prefers dark mode", "category": "preference", "confidence": 0.9}]`

const mergeFactsPrompt = `You have two memory facts about the same topic. Merge them into a single, more complete fact.

EXISTING FACT: %s
NEW FACT: %s

Respond with ONLY the merged fact text, nothing else. Keep it concise but complete.`

// --- Public API ---

// ExtractFacts 使用 LLM 从自然语言内容中提取结构化事实。
func ExtractFacts(ctx context.Context, llm LLMProvider, content string) ([]Fact, error) {
	if content == "" {
		return nil, nil
	}
	if llm == nil {
		// 无 LLM 时退化为单条原始事实
		return []Fact{{Content: content, Category: "fact", Confidence: 0.5}}, nil
	}

	prompt := fmt.Sprintf(extractFactsPrompt, content)
	response, err := llm.Generate(ctx, prompt)
	if err != nil {
		slog.Warn("ExtractFacts LLM 调用失败，退化为原始内容", "error", err)
		return []Fact{{Content: content, Category: "fact", Confidence: 0.5}}, nil
	}

	facts, err := parseFactsJSON(response)
	if err != nil {
		slog.Warn("ExtractFacts JSON 解析失败，退化为原始内容", "error", err)
		return []Fact{{Content: content, Category: "fact", Confidence: 0.5}}, nil
	}

	if len(facts) == 0 {
		return []Fact{{Content: content, Category: "fact", Confidence: 0.5}}, nil
	}

	return facts, nil
}

// DeduplicateFacts 比对新事实与已有记忆，判断是否重复。
// 使用向量相似度进行语义比较，阈值 0.85 以上视为重复。
func DeduplicateFacts(
	ctx context.Context,
	newFacts []Fact,
	userID string,
	vectorStore *VectorStoreService,
	threshold float64,
) *FactPipelineResult {
	if threshold <= 0 {
		threshold = 0.85
	}

	result := &FactPipelineResult{}

	for _, fact := range newFacts {
		if vectorStore == nil {
			result.NewFacts = append(result.NewFacts, fact)
			continue
		}

		// 搜索相似已有记忆
		candidates, err := vectorStore.HybridSearch(ctx, fact.Content, userID, 3, nil, 0, "", nil, nil, nil, nil)
		if err != nil {
			slog.Warn("Dedup 搜索失败，视为新事实", "error", err)
			result.NewFacts = append(result.NewFacts, fact)
			continue
		}

		// 查找超过阈值的最相似记忆
		matched := false
		for _, c := range candidates {
			if c.Score >= threshold {
				// 内容高度相似 — 检查是否完全重复还是需要合并
				if c.Score >= 0.95 {
					// 几乎完全重复，跳过
					result.SkippedCount++
				} else {
					// 相似但有差异，需要合并
					result.UpdatePairs = append(result.UpdatePairs, DedupPair{
						NewFact:      fact,
						ExistingFact: Fact{Content: c.Content},
						ExistingID:   c.MemoryID.String(),
					})
				}
				matched = true
				break
			}
		}

		if !matched {
			result.NewFacts = append(result.NewFacts, fact)
		}
	}

	return result
}

// MergeFacts 使用 LLM 合并冲突的事实对。
func MergeFacts(ctx context.Context, llm LLMProvider, pairs []DedupPair) []DedupPair {
	if llm == nil || len(pairs) == 0 {
		return pairs
	}

	for i := range pairs {
		prompt := fmt.Sprintf(mergeFactsPrompt, pairs[i].ExistingFact.Content, pairs[i].NewFact.Content)
		merged, err := llm.Generate(ctx, prompt)
		if err != nil {
			slog.Warn("MergeFacts LLM 失败，使用新事实覆盖", "error", err)
			continue
		}
		merged = strings.TrimSpace(merged)
		if merged != "" {
			pairs[i].NewFact.Content = merged
		}
	}

	return pairs
}

// --- Internal ---

// parseFactsJSON 从 LLM 响应中解析事实 JSON 数组。
func parseFactsJSON(response string) ([]Fact, error) {
	response = strings.TrimSpace(response)

	// 尝试提取 JSON 数组部分
	start := strings.Index(response, "[")
	end := strings.LastIndex(response, "]")
	if start >= 0 && end > start {
		response = response[start : end+1]
	}

	var facts []Fact
	if err := json.Unmarshal([]byte(response), &facts); err != nil {
		return nil, fmt.Errorf("parse facts JSON: %w", err)
	}

	// 过滤空事实和低置信度
	var valid []Fact
	for _, f := range facts {
		f.Content = strings.TrimSpace(f.Content)
		if f.Content == "" {
			continue
		}
		if f.Confidence <= 0 {
			f.Confidence = 0.5
		}
		if f.Category == "" {
			f.Category = "fact"
		}
		valid = append(valid, f)
	}

	return valid, nil
}
