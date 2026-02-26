package autoreply

import (
	"regexp"
	"strings"
)

// TS 对照: auto-reply/model.ts

// ModelDirectiveResult extractModelDirective 返回值。
type ModelDirectiveResult struct {
	Cleaned      string
	RawModel     string
	RawProfile   string
	HasDirective bool
}

// modelDirectiveRe 匹配 /model 指令。
// TS 对照: model.ts L18-20
var modelDirectiveRe = regexp.MustCompile(`(?i)(?:^|\s)/model(?:$|\s|:)\s*:?\s*([A-Za-z0-9_.:@-]+(?:/[A-Za-z0-9_.:@-]+)*)?\s*`)

// ExtractModelDirective 从消息体提取 /model 指令。
// 返回清理后文本、原始模型/配置文件名、以及是否存在指令。
// TS 对照: model.ts L5-52
func ExtractModelDirective(body string, aliases []string) ModelDirectiveResult {
	if body == "" {
		return ModelDirectiveResult{Cleaned: "", HasDirective: false}
	}

	modelMatch := modelDirectiveRe.FindStringSubmatchIndex(body)

	var aliasMatch []int
	if modelMatch == nil && len(aliases) > 0 {
		var filtered []string
		for _, alias := range aliases {
			trimmed := strings.TrimSpace(alias)
			if trimmed != "" {
				filtered = append(filtered, escapeRegExp(trimmed))
			}
		}
		if len(filtered) > 0 {
			pattern := `(?i)(?:^|\s)/(?:` + strings.Join(filtered, "|") + `)(?:$|\s|:)(?:\s*:\s*)?`
			aliasRe, err := regexp.Compile(pattern)
			if err == nil {
				aliasMatch = aliasRe.FindStringIndex(body)
			}
		}
	}

	var match []int
	var raw string
	if modelMatch != nil {
		match = modelMatch[:2]
		if modelMatch[2] >= 0 && modelMatch[3] >= 0 {
			raw = strings.TrimSpace(body[modelMatch[2]:modelMatch[3]])
		}
	} else if aliasMatch != nil {
		match = aliasMatch
		// 别名匹配时，alias 名本身是 "raw"
		matched := strings.TrimSpace(body[aliasMatch[0]:aliasMatch[1]])
		// 去掉前导空白和 /
		matched = strings.TrimLeft(matched, " \t")
		if strings.HasPrefix(matched, "/") {
			matched = matched[1:]
		}
		// 去掉尾部冒号和空白
		matched = strings.TrimRight(matched, ": \t")
		raw = matched
	}

	rawModel := raw
	var rawProfile string
	if raw != "" && strings.Contains(raw, "@") {
		parts := strings.SplitN(raw, "@", 2)
		rawModel = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			rawProfile = strings.TrimSpace(parts[1])
		}
	}

	var cleaned string
	if match != nil {
		cleaned = body[:match[0]] + " " + body[match[1]:]
		cleaned = collapseWhitespace(cleaned)
	} else {
		cleaned = strings.TrimSpace(body)
	}

	return ModelDirectiveResult{
		Cleaned:      cleaned,
		RawModel:     rawModel,
		RawProfile:   rawProfile,
		HasDirective: match != nil,
	}
}
