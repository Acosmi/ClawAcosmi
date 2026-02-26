package understanding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

// TS 对照: media-understanding/providers/groq.ts (~80L)
// Groq Provider — 音频转录 (Whisper via Groq)

// NewGroqProvider 创建 Groq Provider。
func NewGroqProvider() *Provider {
	return &Provider{
		ID: "groq",
		Capabilities: []Capability{
			{Kind: KindAudioTranscription, Models: []string{"whisper-large-v3-turbo", "whisper-large-v3"}},
		},
		TranscribeAudio: groqTranscribeAudio,
	}
}

// groqTranscribeAudio Groq 音频转录。
// POST /openai/v1/audio/transcriptions (OpenAI-compatible multipart)
func groqTranscribeAudio(req AudioTranscriptionRequest) (*AudioTranscriptionResult, error) {
	model := req.Model
	if model == "" {
		model = DefaultAudioModels["groq"]
	}

	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("groq: 缺少 GROQ_API_KEY 环境变量")
	}

	data, err := readAttachmentData(req.Attachment)
	if err != nil {
		return nil, fmt.Errorf("groq: 读取音频失败: %w", err)
	}

	// 构建 multipart 请求（同 OpenAI 格式）
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", model)
	if req.Language != "" {
		_ = writer.WriteField("language", req.Language)
	}
	_ = writer.WriteField("response_format", "json")

	fileName := "audio.wav"
	if req.Attachment.Path != "" {
		fileName = req.Attachment.Path
	}
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("groq: 创建 form file 失败: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("groq: 写入音频数据失败: %w", err)
	}
	writer.Close()

	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	httpReq, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/audio/transcriptions", &body)
	if err != nil {
		return nil, fmt.Errorf("groq: 创建请求失败: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("groq: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("groq: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("groq: 解析响应失败: %w", err)
	}

	text := result.Text
	if req.MaxChars > 0 && len([]rune(text)) > req.MaxChars {
		text = string([]rune(text)[:req.MaxChars])
	}

	return &AudioTranscriptionResult{
		Text:     text,
		Language: req.Language,
	}, nil
}
