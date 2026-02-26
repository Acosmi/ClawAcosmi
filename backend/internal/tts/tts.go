package tts

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// TS 对照: tts/tts.ts L1200-1580 (主入口)

// ---------- 状态跟踪 ----------

var lastTtsAttemptValue *TtsStatusEntry

// GetLastTtsAttempt 获取最近一次 TTS 尝试状态。
// TS 对照: tts.ts L468-470
func GetLastTtsAttempt() *TtsStatusEntry {
	return lastTtsAttemptValue
}

// SetLastTtsAttempt 设置最近一次 TTS 尝试状态。
// TS 对照: tts.ts L472-474
func SetLastTtsAttempt(entry *TtsStatusEntry) {
	lastTtsAttemptValue = entry
}

// ---------- 主入口 ----------

// SynthesizeTtsParams 合成 TTS 参数。
type SynthesizeTtsParams struct {
	Text      string
	Config    ResolvedTtsConfig
	PrefsPath string
	ChannelID string
	Overrides TtsDirectiveOverrides
}

// SynthesizeTts 主 TTS 合成入口。
// 解析 Provider、输出格式，尝试合成，回退到备用 Provider。
// TS 对照: tts.ts L1200-1380
func SynthesizeTts(params SynthesizeTtsParams) *TtsResult {
	start := time.Now()

	text := strings.TrimSpace(params.Text)
	if text == "" {
		return &TtsResult{
			Success: false,
			Error:   "文本为空",
		}
	}

	// 智能摘要或截断过长文本
	// TS 对照: tts.ts L1493-1523
	text, _ = maybeSummarizeText(text, params.Config.MaxTextLength, params.PrefsPath, params.Config)

	// 确定 Provider
	provider := GetTtsProvider(params.Config, params.PrefsPath)
	if params.Overrides.Provider != "" {
		provider = params.Overrides.Provider
	}

	// 确定输出格式
	outputFmt := ResolveOutputFormat(params.ChannelID)

	// 确定 voice 用于缓存
	voice := resolveVoiceForCache(params.Config, provider, params.Overrides)
	cacheKey := CacheKey(text, provider, voice)

	// 检查缓存
	if cached, ok := GetCached(cacheKey); ok {
		return &TtsResult{
			Success:         true,
			AudioPath:       cached,
			Provider:        string(provider),
			VoiceCompatible: outputFmt.VoiceCompatible,
			LatencyMs:       time.Since(start).Milliseconds(),
		}
	}

	// Provider 尝试顺序
	providerOrder := ResolveTtsProviderOrder(provider)
	for _, p := range providerOrder {
		if !IsTtsProviderConfigured(params.Config, p) {
			continue
		}

		result, err := SynthesizeWithProvider(SynthesizeParams{
			Text:      text,
			Config:    params.Config,
			Provider:  p,
			OutputFmt: outputFmt,
			TimeoutMs: params.Config.TimeoutMs,
			Overrides: params.Overrides,
		})
		if err != nil {
			log.Printf("[tts] Provider %s 失败: %v", p, err)
			continue
		}
		if result != nil && result.Success {
			result.VoiceCompatible = outputFmt.VoiceCompatible
			if result.AudioPath != "" {
				SetCached(cacheKey, result.AudioPath)
				ScheduleCleanup(result.AudioPath, 0)
			}
			// 记录状态
			SetLastTtsAttempt(&TtsStatusEntry{
				Timestamp:  time.Now().UnixMilli(),
				Success:    true,
				TextLength: len([]rune(text)),
				Provider:   string(p),
				LatencyMs:  time.Since(start).Milliseconds(),
			})
			return result
		}
	}

	// 所有 Provider 失败
	errMsg := fmt.Sprintf("所有 TTS Provider 均失败 (尝试: %v)", providerOrder)
	SetLastTtsAttempt(&TtsStatusEntry{
		Timestamp:  time.Now().UnixMilli(),
		Success:    false,
		TextLength: len([]rune(text)),
		Error:      errMsg,
		LatencyMs:  time.Since(start).Milliseconds(),
	})
	return &TtsResult{
		Success:   false,
		Error:     errMsg,
		LatencyMs: time.Since(start).Milliseconds(),
	}
}

// resolveVoiceForCache 解析用于缓存键的语音标识。
func resolveVoiceForCache(config ResolvedTtsConfig, provider TtsProvider, overrides TtsDirectiveOverrides) string {
	switch provider {
	case ProviderOpenAI:
		voice := config.OpenAI.Voice
		if overrides.OpenAI != nil && overrides.OpenAI.Voice != "" {
			voice = overrides.OpenAI.Voice
		}
		return voice
	case ProviderElevenLabs:
		voice := config.ElevenLabs.VoiceID
		if overrides.ElevenLabs != nil && overrides.ElevenLabs.VoiceID != "" {
			voice = overrides.ElevenLabs.VoiceID
		}
		return voice
	case ProviderEdge:
		return config.Edge.Voice
	default:
		return ""
	}
}

// ---------- 系统提示辅助 ----------

// BuildTtsSystemPromptHint 构建 TTS 系统提示。
// TS 对照: tts.ts L343-366
func BuildTtsSystemPromptHint(config ResolvedTtsConfig, prefsPath string) string {
	autoMode := ResolveTtsAutoMode(config, prefsPath, "")
	if autoMode == AutoOff {
		return ""
	}
	maxLength := GetTtsMaxLength(prefsPath)
	summarize := "on"
	if !IsSummarizationEnabled(prefsPath) {
		summarize = "off"
	}

	var lines []string
	lines = append(lines, "Voice (TTS) is enabled.")

	switch autoMode {
	case AutoInbound:
		lines = append(lines, "Only use TTS when the user's last message includes audio/voice.")
	case AutoTagged:
		lines = append(lines, "Only use TTS when you include [[tts]] or [[tts:text]] tags.")
	}

	lines = append(lines, fmt.Sprintf(
		"Keep spoken text ≤%d chars to avoid auto-summary (summary %s).", maxLength, summarize,
	))
	lines = append(lines,
		"Use [[tts:...]] and optional [[tts:text]]...[[/tts:text]] to control voice/expressiveness.",
	)

	return strings.Join(lines, "\n")
}

// ---------- ApplyTts (统一应用入口) ----------

// ApplyTtsParams 应用 TTS 参数。
type ApplyTtsParams struct {
	Text      string
	Config    ResolvedTtsConfig
	PrefsPath string
	ChannelID string
	// 如果为 true，自动解析 TTS 指令
	ParseDirectives bool
}

// ApplyTts 完整 TTS 应用流程：解析指令 → 合成。
// TS 对照: tts.ts L1380-1580
func ApplyTts(params ApplyTtsParams) *TtsResult {
	text := params.Text
	var overrides TtsDirectiveOverrides

	if params.ParseDirectives {
		parsed := ParseTtsDirectives(text, params.Config.ModelOverrides)
		text = parsed.CleanedText
		overrides = parsed.Overrides
		if parsed.TtsText != "" {
			text = parsed.TtsText
		}
	}

	return SynthesizeTts(SynthesizeTtsParams{
		Text:      text,
		Config:    params.Config,
		PrefsPath: params.PrefsPath,
		ChannelID: params.ChannelID,
		Overrides: overrides,
	})
}
