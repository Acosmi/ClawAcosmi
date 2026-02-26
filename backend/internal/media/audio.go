package media

import "strings"

// TS 对照: media/audio.ts (23L)

// voiceCompatibleExts 语音兼容的音频扩展名（OGG/Opus 系）。
// TS 对照: audio.ts L3 VOICE_AUDIO_EXTENSIONS
var voiceCompatibleExts = map[string]bool{
	".oga":  true,
	".ogg":  true,
	".opus": true,
}

// IsVoiceCompatibleAudio 判断音频文件是否为语音兼容格式（OGG/Opus）。
// 用于 Telegram 等平台判断是否作为语音消息发送。
// TS 对照: audio.ts L5-22
func IsVoiceCompatibleAudio(contentType, fileName string) bool {
	if contentType != "" {
		mime := normalizeHeaderMime(contentType)
		if containsAny(mime, "ogg", "opus") {
			return true
		}
	}
	fn := strings.TrimSpace(fileName)
	if fn == "" {
		return false
	}
	ext := GetFileExtension(fn)
	if ext == "" {
		return false
	}
	return voiceCompatibleExts[ext]
}

// containsAny 检查 s 是否包含 substrs 中的任一子串。
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
