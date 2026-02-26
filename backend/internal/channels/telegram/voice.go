package telegram

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// voiceCompatibleExtensions 是 Telegram 语音消息支持的音频扩展名。
var voiceCompatibleExtensions = map[string]bool{
	".oga":  true,
	".ogg":  true,
	".opus": true,
}

// VoiceDecision 表示语音发送决策结果。
type VoiceDecision struct {
	UseVoice bool
	Reason   string // 不使用语音的原因（空表示无原因）
}

// IsVoiceCompatibleAudio 判断媒体是否兼容 Telegram 语音消息格式。
// 继承自 media/audio.ts 的 isVoiceCompatibleAudio 逻辑。
func IsVoiceCompatibleAudio(contentType, fileName string) bool {
	mime := strings.ToLower(contentType)
	if mime != "" && (strings.Contains(mime, "ogg") || strings.Contains(mime, "opus")) {
		return true
	}
	name := strings.TrimSpace(fileName)
	if name == "" {
		return false
	}
	ext := getVoiceFileExtension(name)
	return voiceCompatibleExtensions[ext]
}

// getVoiceFileExtension 从文件路径或 URL 中提取扩展名。
// 对齐 TS: 如果是 URL，从 pathname 提取扩展名（忽略查询参数）。
func getVoiceFileExtension(filePath string) string {
	lower := strings.ToLower(filePath)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		if u, err := url.Parse(filePath); err == nil {
			return strings.ToLower(filepath.Ext(u.Path))
		}
	}
	return strings.ToLower(filepath.Ext(filePath))
}

// ResolveTelegramVoiceDecision 决定是否使用语音发送。
func ResolveTelegramVoiceDecision(wantsVoice bool, contentType, fileName string) VoiceDecision {
	if !wantsVoice {
		return VoiceDecision{UseVoice: false}
	}
	if IsVoiceCompatibleAudio(contentType, fileName) {
		return VoiceDecision{UseVoice: true}
	}
	ct := contentType
	if ct == "" {
		ct = "unknown"
	}
	fn := fileName
	if fn == "" {
		fn = "unknown"
	}
	return VoiceDecision{
		UseVoice: false,
		Reason:   fmt.Sprintf("media is %s (%s)", ct, fn),
	}
}

// ResolveTelegramVoiceSend 决定是否使用语音发送，并在需要时记录回退日志。
func ResolveTelegramVoiceSend(wantsVoice bool, contentType, fileName string, logFallback func(string)) VoiceDecision {
	decision := ResolveTelegramVoiceDecision(wantsVoice, contentType, fileName)
	if decision.Reason != "" && logFallback != nil {
		logFallback(fmt.Sprintf(
			"Telegram voice requested but %s; sending as audio file instead.",
			decision.Reason,
		))
	}
	return VoiceDecision{UseVoice: decision.UseVoice}
}
