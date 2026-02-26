// Package understanding 提供媒体理解能力（音频转录、视频描述、图像描述）。
//
// TS 对照: media-understanding/ 目录 (44 文件, ~3000L)
//
// 主要功能:
//   - 多 Provider 注册表（OpenAI/Google/Anthropic/Deepgram/Groq/MiniMax）
//   - 能力路由与模型解析
//   - 并发执行控制
//   - 作用域决策
package understanding

// ---------- 媒体理解种类 ----------

// Kind 媒体理解能力种类。
// TS 对照: types.ts L1-4
type Kind string

const (
	KindAudioTranscription Kind = "audio.transcription"
	KindVideoDescription   Kind = "video.description"
	KindImageDescription   Kind = "image.description"
)

// ---------- 媒体附件 ----------

// MediaAttachment 待处理的媒体附件。
// TS 对照: types.ts L6-11
type MediaAttachment struct {
	Path  string `json:"path,omitempty"`
	URL   string `json:"url,omitempty"`
	MIME  string `json:"mime,omitempty"`
	Index int    `json:"index"`
}

// ---------- 能力 ----------

// Capability Provider 能力声明。
// TS 对照: types.ts L16-20
type Capability struct {
	Kind   Kind     `json:"kind"`
	Models []string `json:"models,omitempty"`
}

// ---------- 请求/结果类型 ----------

// AudioTranscriptionRequest 音频转录请求。
// TS 对照: types.ts L40-52
type AudioTranscriptionRequest struct {
	Attachment MediaAttachment `json:"attachment"`
	Model      string          `json:"model,omitempty"`
	Language   string          `json:"language,omitempty"`
	Prompt     string          `json:"prompt,omitempty"`
	MaxChars   int             `json:"maxChars,omitempty"`
	TimeoutMs  int             `json:"timeoutMs,omitempty"`
}

// AudioTranscriptionResult 音频转录结果。
// TS 对照: types.ts L54-58
type AudioTranscriptionResult struct {
	Text       string `json:"text"`
	Language   string `json:"language,omitempty"`
	DurationMs int    `json:"durationMs,omitempty"`
}

// VideoDescriptionRequest 视频描述请求。
// TS 对照: types.ts L60-72
type VideoDescriptionRequest struct {
	Attachment     MediaAttachment `json:"attachment"`
	Model          string          `json:"model,omitempty"`
	Prompt         string          `json:"prompt,omitempty"`
	MaxChars       int             `json:"maxChars,omitempty"`
	MaxBase64Bytes int             `json:"maxBase64Bytes,omitempty"`
	TimeoutMs      int             `json:"timeoutMs,omitempty"`
}

// VideoDescriptionResult 视频描述结果。
// TS 对照: types.ts L74-77
type VideoDescriptionResult struct {
	Text       string `json:"text"`
	DurationMs int    `json:"durationMs,omitempty"`
}

// ImageDescriptionRequest 图像描述请求。
// TS 对照: types.ts L79-90
type ImageDescriptionRequest struct {
	Attachment MediaAttachment `json:"attachment"`
	Model      string          `json:"model,omitempty"`
	Prompt     string          `json:"prompt,omitempty"`
	MaxChars   int             `json:"maxChars,omitempty"`
	MaxBytes   int             `json:"maxBytes,omitempty"`
	TimeoutMs  int             `json:"timeoutMs,omitempty"`
}

// ImageDescriptionResult 图像描述结果。
// TS 对照: types.ts L92-94
type ImageDescriptionResult struct {
	Text string `json:"text"`
}

// ---------- Provider 接口 ----------

// Provider 媒体理解 Provider 接口。
// TS 对照: types.ts L100-115
type Provider struct {
	ID              string
	Capabilities    []Capability
	TranscribeAudio func(req AudioTranscriptionRequest) (*AudioTranscriptionResult, error)
	DescribeVideo   func(req VideoDescriptionRequest) (*VideoDescriptionResult, error)
	DescribeImage   func(req ImageDescriptionRequest) (*ImageDescriptionResult, error)
}

// ---------- 理解输出 ----------

// Output 媒体理解输出。
// TS 对照: types.ts L22-32
type Output struct {
	Kind       Kind            `json:"kind"`
	Provider   string          `json:"provider"`
	Model      string          `json:"model,omitempty"`
	Attachment MediaAttachment `json:"attachment"`
	Text       string          `json:"text"`
	Error      string          `json:"error,omitempty"`
	DurationMs int             `json:"durationMs,omitempty"`
}

// Decision 媒体理解决策（是否运行某个能力）。
// TS 对照: types.ts L34-38
type Decision struct {
	Kind    Kind   `json:"kind"`
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}
