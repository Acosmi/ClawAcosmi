package understanding

// TS 对照: media-understanding/defaults.ts (37L)

const (
	// DefaultMaxChars 默认最大字符数。
	DefaultMaxChars = 4000

	// DefaultMaxBytes 默认最大字节数 (16MB)。
	DefaultMaxBytes = 16 * 1024 * 1024

	// DefaultTimeoutMs 默认超时 (30 秒)。
	DefaultTimeoutMs = 30_000

	// DefaultVideoMaxBase64Bytes 默认视频 Base64 最大字节数 (10MB)。
	DefaultVideoMaxBase64Bytes = 10 * 1024 * 1024
)

// DefaultPrompt 默认 prompt。
// TS 对照: defaults.ts L15
const DefaultPrompt = "Describe what you see or hear in detail."

// DefaultAudioTranscriptionPrompt 默认音频转录 prompt。
const DefaultAudioTranscriptionPrompt = "Transcribe the audio accurately."

// DefaultAudioModels 默认音频转录模型映射。
// TS 对照: defaults.ts L20-30
var DefaultAudioModels = map[string]string{
	"openai":   "whisper-1",
	"groq":     "whisper-large-v3-turbo",
	"deepgram": "nova-2",
	"google":   "gemini-2.0-flash",
}

// DefaultVideoModels 默认视频描述模型映射。
var DefaultVideoModels = map[string]string{
	"google": "gemini-2.0-flash",
}

// DefaultImageModels 默认图像描述模型映射。
var DefaultImageModels = map[string]string{
	"openai":    "gpt-4o-mini",
	"anthropic": "claude-3-haiku-20240307",
	"google":    "gemini-2.0-flash",
	"minimax":   "abab6.5s-chat",
}
