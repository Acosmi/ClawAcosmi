package uhms

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// ---------- extractMemoriesHeuristic ----------

func TestExtractMemoriesHeuristic_Preference(t *testing.T) {
	text := "[user]: 我喜欢深色主题，偏好 Vim 编辑器\n\n[assistant]: 好的，已记住\n"
	results := extractMemoriesHeuristic(text)
	if len(results) == 0 {
		t.Fatal("expected at least 1 memory extracted")
	}
	if results[0].Category != CatPreference {
		t.Errorf("expected category preference, got %s", results[0].Category)
	}
	if !strings.Contains(results[0].Content, "深色主题") {
		t.Errorf("expected content to contain '深色主题', got %q", results[0].Content)
	}
}

func TestExtractMemoriesHeuristic_Profile(t *testing.T) {
	text := "[user]: 我是一名前端开发者，我在北京工作\n\n[assistant]: 了解\n"
	results := extractMemoriesHeuristic(text)
	if len(results) == 0 {
		t.Fatal("expected at least 1 memory extracted")
	}
	if results[0].Category != CatProfile {
		t.Errorf("expected category profile, got %s", results[0].Category)
	}
}

func TestExtractMemoriesHeuristic_Goal(t *testing.T) {
	text := "[user]: 我的目标是学习 Rust，计划今年完成一个开源项目\n"
	results := extractMemoriesHeuristic(text)
	if len(results) == 0 {
		t.Fatal("expected at least 1 memory extracted")
	}
	if results[0].Category != CatGoal {
		t.Errorf("expected category goal, got %s", results[0].Category)
	}
}

func TestExtractMemoriesHeuristic_Skill(t *testing.T) {
	text := "[user]: 我擅长 Go 和 TypeScript，我熟悉 Docker\n"
	results := extractMemoriesHeuristic(text)
	if len(results) == 0 {
		t.Fatal("expected at least 1 memory extracted")
	}
	if results[0].Category != CatSkill {
		t.Errorf("expected category skill, got %s", results[0].Category)
	}
}

func TestExtractMemoriesHeuristic_ShortText(t *testing.T) {
	text := "hi"
	results := extractMemoriesHeuristic(text)
	if len(results) != 0 {
		t.Errorf("expected 0 results for short text, got %d", len(results))
	}
}

func TestExtractMemoriesHeuristic_SystemLineFiltered(t *testing.T) {
	text := "[system]: 系统初始化完成\n[assistant]: 你好\n"
	results := extractMemoriesHeuristic(text)
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-user lines, got %d", len(results))
	}
}

func TestExtractMemoriesHeuristic_Dedup_Chinese(t *testing.T) {
	text := "[user]: 我喜欢深色主题\n\n[user]: 我喜欢深色主题和暗色模式\n"
	results := extractMemoriesHeuristic(text)
	if len(results) > 1 {
		t.Errorf("expected dedup to prevent Chinese duplicates, got %d results", len(results))
	}
}

func TestExtractMemoriesHeuristic_Dedup_English(t *testing.T) {
	text := "[user]: I prefer dark mode and always use VS Code\n\n[user]: I prefer dark mode and always use VS Code editor\n"
	results := extractMemoriesHeuristic(text)
	if len(results) > 1 {
		t.Errorf("expected dedup to prevent English duplicates, got %d results", len(results))
	}
}

func TestExtractMemoriesHeuristic_MaxLimit(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 10; i++ {
		sb.WriteString("[user]: 我喜欢工具" + string(rune('A'+i)) + "，偏好使用它" + string(rune('0'+i)) + "\n\n")
	}
	results := extractMemoriesHeuristic(sb.String())
	if len(results) > maxHeuristicExtract {
		t.Errorf("expected max %d results, got %d", maxHeuristicExtract, len(results))
	}
}

func TestExtractMemoriesHeuristic_ScoreThreshold(t *testing.T) {
	text := "[user]: 今天天气不错\n"
	results := extractMemoriesHeuristic(text)
	if len(results) != 0 {
		t.Errorf("expected 0 results for text below score threshold, got %d", len(results))
	}
}

// ---------- classifyByKeywords ----------

func TestClassifyByKeywords_Preference(t *testing.T) {
	cat, score := classifyByKeywords("我喜欢使用 Vim 编辑器")
	if cat != CatPreference {
		t.Errorf("expected preference, got %s", cat)
	}
	if score < 2.0 {
		t.Errorf("expected score >= 2.0, got %f", score)
	}
}

func TestClassifyByKeywords_EnglishPreference(t *testing.T) {
	cat, score := classifyByKeywords("I prefer dark mode and always use VS Code")
	if cat != CatPreference {
		t.Errorf("expected preference, got %s", cat)
	}
	if score < 2.0 {
		t.Errorf("expected score >= 2.0, got %f", score)
	}
}

func TestClassifyByKeywords_NoMatch(t *testing.T) {
	_, score := classifyByKeywords("hello world")
	if score >= 2.0 {
		t.Errorf("expected score < 2.0 for generic text, got %f", score)
	}
}

// ---------- failingLLM mock ----------

type failingLLM struct{}

func (f *failingLLM) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "", fmt.Errorf("mock: LLM unavailable")
}
func (f *failingLLM) EstimateTokens(text string) int { return len(text) / 4 }

// ---------- NilLLM 走 fallback ----------

func TestExtractMemoriesFromTranscript_NilLLM(t *testing.T) {
	// 当 m.llm == nil 时应走 heuristic fallback（通过真实函数调用验证）
	m := &DefaultManager{llm: nil}
	text := "[user]: 我喜欢深色主题，偏好 VS Code\n\n[assistant]: 好的\n"
	results := extractMemoriesFromTranscript(context.Background(), m, text)
	if len(results) == 0 {
		t.Fatal("nil-LLM fallback should extract at least 1 memory")
	}
}

// ---------- LLM 失败走 heuristic fallback (Bug#11 P1) ----------

func TestExtractMemoriesFromTranscript_LLMFailFallback(t *testing.T) {
	// 注入返回 error 的 mock LLM，验证 extractMemoriesFromTranscript 降级到 heuristic
	m := &DefaultManager{llm: &failingLLM{}}
	text := "[user]: 我喜欢深色主题，偏好 Vim 编辑器\n\n[assistant]: 好的，已记住\n"
	results := extractMemoriesFromTranscript(context.Background(), m, text)
	if len(results) == 0 {
		t.Fatal("LLM failure fallback should produce at least 1 memory via heuristic")
	}
	if results[0].Category != CatPreference {
		t.Errorf("expected preference category, got %s", results[0].Category)
	}
}

func TestExtractMemoriesHeuristic_EmptyText(t *testing.T) {
	results := extractMemoriesHeuristic("")
	if results != nil && len(results) != 0 {
		t.Errorf("expected nil/empty for empty text, got %d results", len(results))
	}
}
