package tts

import (
	"fmt"
	"regexp"
	"strings"
)

// TS 对照: tts/tts.ts L553-780 (TTS 指令解析)

// ---------- 正则表达式 ----------

// ttsTagOpenRe 匹配 [[tts:...]] 或 [[tts]] 开始标签。
var ttsTagOpenRe = regexp.MustCompile(`(?i)\[\[\s*tts(?::([^\]]*))?\s*\]\]`)

// ttsTextBlockRe 匹配 [[tts:text]]...[[/tts:text]] 块。
var ttsTextBlockRe = regexp.MustCompile(`(?is)\[\[\s*tts:text\s*\]\](.*?)\[\[/\s*tts:text\s*\]\]`)

// ttsProviderRe 匹配 provider=<value> 参数。
var ttsProviderRe = regexp.MustCompile(`(?i)(?:^|\s)provider=(\S+)`)

// ttsVoiceRe 匹配 voice=<value> 参数。
var ttsVoiceRe = regexp.MustCompile(`(?i)(?:^|\s)voice=(\S+)`)

// ttsModelRe 匹配 model=<value> 参数。
var ttsModelRe = regexp.MustCompile(`(?i)(?:^|\s)model=(\S+)`)

// ttsSeedRe 匹配 seed=<value> 参数。
var ttsSeedRe = regexp.MustCompile(`(?i)(?:^|\s)seed=(\d+)`)

// ---------- 解析函数 ----------

// ParseTtsDirectives 解析文本中的 TTS 指令。
// TS 对照: tts.ts L553-780
func ParseTtsDirectives(text string, overrides ResolvedTtsModelOverrides) TtsDirectiveParseResult {
	result := TtsDirectiveParseResult{
		CleanedText: text,
		Overrides:   TtsDirectiveOverrides{},
	}

	if strings.TrimSpace(text) == "" {
		return result
	}

	// 1. 提取 [[tts:text]]...[[/tts:text]] 块
	textBlockMatch := ttsTextBlockRe.FindStringSubmatchIndex(text)
	if len(textBlockMatch) >= 4 {
		ttsTextContent := strings.TrimSpace(text[textBlockMatch[2]:textBlockMatch[3]])
		if overrides.AllowText && ttsTextContent != "" {
			result.Overrides.TtsText = ttsTextContent
			result.TtsText = ttsTextContent
		}
		// 移除 text block
		text = text[:textBlockMatch[0]] + text[textBlockMatch[1]:]
		result.HasDirective = true
	}

	// 2. 提取 [[tts:...]] 标签
	tagMatches := ttsTagOpenRe.FindAllStringSubmatchIndex(text, -1)
	if len(tagMatches) == 0 {
		result.CleanedText = cleanTtsWhitespace(text)
		return result
	}

	result.HasDirective = true

	// 从最后一个匹配开始移除，保持索引正确
	cleaned := text
	for i := len(tagMatches) - 1; i >= 0; i-- {
		match := tagMatches[i]
		fullStart := match[0]
		fullEnd := match[1]

		// 提取参数（如果有）
		if match[2] >= 0 && match[3] >= 0 {
			params := strings.TrimSpace(text[match[2]:match[3]])
			parseDirectiveParams(params, overrides, &result)
		}

		cleaned = cleaned[:fullStart] + cleaned[fullEnd:]
	}

	result.CleanedText = cleanTtsWhitespace(cleaned)
	return result
}

// parseDirectiveParams 解析指令参数。
func parseDirectiveParams(params string, overrides ResolvedTtsModelOverrides, result *TtsDirectiveParseResult) {
	if params == "" {
		return
	}

	// provider=xxx
	if m := ttsProviderRe.FindStringSubmatch(params); len(m) > 1 {
		if overrides.AllowProvider {
			p := TtsProvider(strings.ToLower(m[1]))
			switch p {
			case ProviderOpenAI, ProviderElevenLabs, ProviderEdge:
				result.Overrides.Provider = p
			default:
				result.Warnings = append(result.Warnings, "未知 TTS provider: "+m[1])
			}
		}
	}

	// voice=xxx
	if m := ttsVoiceRe.FindStringSubmatch(params); len(m) > 1 {
		if overrides.AllowVoice {
			voice := m[1]
			// 根据当前或覆盖的 provider 分配 voice
			provider := result.Overrides.Provider
			if provider == ProviderElevenLabs || (provider == "" && IsValidVoiceID(voice)) {
				if result.Overrides.ElevenLabs == nil {
					result.Overrides.ElevenLabs = &TtsDirectiveELOverride{}
				}
				result.Overrides.ElevenLabs.VoiceID = voice
			} else {
				if result.Overrides.OpenAI == nil {
					result.Overrides.OpenAI = &TtsDirectiveOpenAIOverride{}
				}
				result.Overrides.OpenAI.Voice = voice
			}
		}
	}

	// model=xxx
	if m := ttsModelRe.FindStringSubmatch(params); len(m) > 1 {
		if overrides.AllowModelID {
			model := m[1]
			provider := result.Overrides.Provider
			if provider == ProviderElevenLabs {
				if result.Overrides.ElevenLabs == nil {
					result.Overrides.ElevenLabs = &TtsDirectiveELOverride{}
				}
				result.Overrides.ElevenLabs.ModelID = model
			} else {
				if result.Overrides.OpenAI == nil {
					result.Overrides.OpenAI = &TtsDirectiveOpenAIOverride{}
				}
				result.Overrides.OpenAI.Model = model
			}
		}
	}

	// seed=xxx (ElevenLabs)
	if m := ttsSeedRe.FindStringSubmatch(params); len(m) > 1 {
		if overrides.AllowSeed {
			var seed int
			_, _ = fmt.Sscanf(m[1], "%d", &seed)
			if result.Overrides.ElevenLabs == nil {
				result.Overrides.ElevenLabs = &TtsDirectiveELOverride{}
			}
			result.Overrides.ElevenLabs.Seed = &seed
		}
	}
}

// cleanTtsWhitespace 清理 TTS 指令移除后的多余空白。
func cleanTtsWhitespace(s string) string {
	// 合并连续空行
	nlRe := regexp.MustCompile(`\n{3,}`)
	s = nlRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
