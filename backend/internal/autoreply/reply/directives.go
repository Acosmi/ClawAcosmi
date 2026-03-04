package reply

import (
	"regexp"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
)

// TS 对照: auto-reply/reply/directives.ts (194L)

// ---------- 通用指令提取 ----------

// extractedLevel 指令提取结果。
type extractedLevel[T ~string] struct {
	Cleaned      string
	Level        T
	RawLevel     string
	HasDirective bool
	LevelSet     bool // Go 无 undefined，用此标记 Level 是否已赋值
}

func escapeRegExpForDirective(value string) string {
	return regexp.QuoteMeta(value)
}

// matchLevelDirective 匹配 /directive 格式的指令。
func matchLevelDirective(body string, names []string) (start, end int, rawLevel string, found bool) {
	namePattern := strings.Join(names, "|")
	re, err := regexp.Compile(`(?i)(?:^|\s)/(?:` + namePattern + `)(?:$|\s|:)`)
	if err != nil {
		return 0, 0, "", false
	}
	loc := re.FindStringIndex(body)
	if loc == nil {
		return 0, 0, "", false
	}
	start = loc[0]
	end = loc[1]

	i := end
	// 跳过空白
	for i < len(body) && (body[i] == ' ' || body[i] == '\t' || body[i] == '\n' || body[i] == '\r') {
		i++
	}
	// 跳过冒号
	if i < len(body) && body[i] == ':' {
		i++
		for i < len(body) && (body[i] == ' ' || body[i] == '\t') {
			i++
		}
	}
	// 读取参数
	argStart := i
	for i < len(body) && ((body[i] >= 'A' && body[i] <= 'Z') || (body[i] >= 'a' && body[i] <= 'z') || body[i] == '-') {
		i++
	}
	if i > argStart {
		rawLevel = body[argStart:i]
	}
	end = i
	return start, end, rawLevel, true
}

// extractLevelDirective 通用指令提取函数。
func extractLevelDirective[T ~string](body string, names []string, normalize func(string) (T, bool)) extractedLevel[T] {
	start, end, rawLevel, found := matchLevelDirective(body, names)
	if !found {
		return extractedLevel[T]{Cleaned: strings.TrimSpace(body), HasDirective: false}
	}
	level, ok := normalize(rawLevel)
	cleaned := body[:start] + " " + body[end:]
	cleaned = collapseWhitespace(cleaned)
	return extractedLevel[T]{
		Cleaned:      cleaned,
		Level:        level,
		RawLevel:     rawLevel,
		HasDirective: true,
		LevelSet:     ok,
	}
}

// extractSimpleDirective 简单指令提取（无参数）。
func extractSimpleDirective(body string, names []string) (cleaned string, hasDirective bool) {
	namePattern := strings.Join(names, "|")
	re, err := regexp.Compile(`(?i)(?:^|\s)/(?:` + namePattern + `)(?:$|\s|:)(?:\s*:\s*)?`)
	if err != nil {
		return strings.TrimSpace(body), false
	}
	loc := re.FindStringIndex(body)
	if loc == nil {
		return strings.TrimSpace(body), false
	}
	result := body[:loc[0]] + " " + body[loc[1]:]
	return collapseWhitespace(result), true
}

// collapseWhitespace 合并多余空白。
func collapseWhitespace(s string) string {
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(s, " "))
}

// ---------- 具体指令提取函数 ----------

// ThinkDirectiveResult /think 指令提取结果。
type ThinkDirectiveResult struct {
	Cleaned      string
	ThinkLevel   autoreply.ThinkLevel
	RawLevel     string
	HasDirective bool
	LevelSet     bool
}

// ExtractThinkDirective 从消息体提取 /think 指令。
// TS 对照: directives.ts L92-108
func ExtractThinkDirective(body string) ThinkDirectiveResult {
	if body == "" {
		return ThinkDirectiveResult{Cleaned: "", HasDirective: false}
	}
	r := extractLevelDirective(body, []string{"thinking", "think", "t"}, func(raw string) (autoreply.ThinkLevel, bool) {
		return autoreply.NormalizeThinkLevel(raw)
	})
	return ThinkDirectiveResult{
		Cleaned:      r.Cleaned,
		ThinkLevel:   r.Level,
		RawLevel:     r.RawLevel,
		HasDirective: r.HasDirective,
		LevelSet:     r.LevelSet,
	}
}

// VerboseDirectiveResult /verbose 指令提取结果。
type VerboseDirectiveResult struct {
	Cleaned      string
	VerboseLevel autoreply.VerboseLevel
	RawLevel     string
	HasDirective bool
	LevelSet     bool
}

// ExtractVerboseDirective 从消息体提取 /verbose 指令。
// TS 对照: directives.ts L110-126
func ExtractVerboseDirective(body string) VerboseDirectiveResult {
	if body == "" {
		return VerboseDirectiveResult{Cleaned: "", HasDirective: false}
	}
	r := extractLevelDirective(body, []string{"verbose", "v"}, func(raw string) (autoreply.VerboseLevel, bool) {
		return autoreply.NormalizeVerboseLevel(raw)
	})
	return VerboseDirectiveResult{
		Cleaned:      r.Cleaned,
		VerboseLevel: r.Level,
		RawLevel:     r.RawLevel,
		HasDirective: r.HasDirective,
		LevelSet:     r.LevelSet,
	}
}

// ElevatedDirectiveResult /elevated 指令提取结果。
type ElevatedDirectiveResult struct {
	Cleaned       string
	ElevatedLevel autoreply.ElevatedLevel
	RawLevel      string
	HasDirective  bool
	LevelSet      bool
}

// ExtractElevatedDirective 从消息体提取 /elevated 指令。
// TS 对照: directives.ts L146-162
func ExtractElevatedDirective(body string) ElevatedDirectiveResult {
	if body == "" {
		return ElevatedDirectiveResult{Cleaned: "", HasDirective: false}
	}
	r := extractLevelDirective(body, []string{"elevated", "elev"}, func(raw string) (autoreply.ElevatedLevel, bool) {
		return autoreply.NormalizeElevatedLevel(raw)
	})
	return ElevatedDirectiveResult{
		Cleaned:       r.Cleaned,
		ElevatedLevel: r.Level,
		RawLevel:      r.RawLevel,
		HasDirective:  r.HasDirective,
		LevelSet:      r.LevelSet,
	}
}

// ReasoningDirectiveResult /reasoning 指令提取结果。
type ReasoningDirectiveResult struct {
	Cleaned        string
	ReasoningLevel autoreply.ReasoningLevel
	RawLevel       string
	HasDirective   bool
	LevelSet       bool
}

// ExtractReasoningDirective 从消息体提取 /reasoning 指令。
// TS 对照: directives.ts L164-180
func ExtractReasoningDirective(body string) ReasoningDirectiveResult {
	if body == "" {
		return ReasoningDirectiveResult{Cleaned: "", HasDirective: false}
	}
	r := extractLevelDirective(body, []string{"reasoning", "reason"}, func(raw string) (autoreply.ReasoningLevel, bool) {
		return autoreply.NormalizeReasoningLevel(raw)
	})
	return ReasoningDirectiveResult{
		Cleaned:        r.Cleaned,
		ReasoningLevel: r.Level,
		RawLevel:       r.RawLevel,
		HasDirective:   r.HasDirective,
		LevelSet:       r.LevelSet,
	}
}

// StatusDirectiveResult /status 简单指令提取结果。
type StatusDirectiveResult struct {
	Cleaned      string
	HasDirective bool
}

// ExtractStatusDirective 从消息体提取 /status 指令。
// TS 对照: directives.ts L182-190
func ExtractStatusDirective(body string) StatusDirectiveResult {
	if body == "" {
		return StatusDirectiveResult{Cleaned: "", HasDirective: false}
	}
	cleaned, has := extractSimpleDirective(body, []string{"status"})
	return StatusDirectiveResult{Cleaned: cleaned, HasDirective: has}
}
