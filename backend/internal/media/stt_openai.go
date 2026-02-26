package media

// stt_openai.go — OpenAI Whisper STT 实现（Phase C 新增）
// 兼容 OpenAI / Groq / Azure 等 OpenAI API 格式的 STT 服务
// API: POST /v1/audio/transcriptions (multipart/form-data)

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// OpenAISTT OpenAI Whisper API 实现
type OpenAISTT struct {
	apiKey  string
	model   string
	baseURL string
	lang    string
}

// NewOpenAISTT 创建 OpenAI STT Provider
func NewOpenAISTT(cfg *types.STTConfig) *OpenAISTT {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := cfg.Model
	if model == "" {
		model = "whisper-1"
	}
	return &OpenAISTT{
		apiKey:  cfg.APIKey,
		model:   model,
		baseURL: baseURL,
		lang:    cfg.Language,
	}
}

// Name 返回 Provider 名称
func (s *OpenAISTT) Name() string {
	return "openai"
}

// Transcribe 调用 OpenAI Whisper API 转录音频
func (s *OpenAISTT) Transcribe(ctx context.Context, audioData []byte, mimeType string) (string, error) {
	if len(audioData) == 0 {
		return "", fmt.Errorf("stt/openai: empty audio data")
	}

	// 根据 MIME 类型确定文件扩展名
	ext := mimeTypeToExt(mimeType)

	// 构建 multipart 请求
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// file 字段
	part, err := writer.CreateFormFile("file", "audio"+ext)
	if err != nil {
		return "", fmt.Errorf("stt/openai: create form file: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return "", fmt.Errorf("stt/openai: write audio data: %w", err)
	}

	// model 字段
	if err := writer.WriteField("model", s.model); err != nil {
		return "", fmt.Errorf("stt/openai: write model field: %w", err)
	}

	// language 字段（可选）
	if s.lang != "" {
		if err := writer.WriteField("language", s.lang); err != nil {
			return "", fmt.Errorf("stt/openai: write language field: %w", err)
		}
	}

	// response_format 字段
	if err := writer.WriteField("response_format", "json"); err != nil {
		return "", fmt.Errorf("stt/openai: write response_format: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("stt/openai: close multipart: %w", err)
	}

	// HTTP 请求
	url := s.baseURL + "/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return "", fmt.Errorf("stt/openai: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("stt/openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("stt/openai: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("stt/openai: API error: status=%d body=%s",
			resp.StatusCode, truncateString(string(body), 500))
	}

	// 解析 JSON 响应
	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("stt/openai: parse response: %w", err)
	}

	slog.Info("stt/openai: transcription complete",
		"model", s.model,
		"audio_size", len(audioData),
		"text_len", len(result.Text),
	)
	return result.Text, nil
}

// TestConnection 测试 API 连接
func (s *OpenAISTT) TestConnection(ctx context.Context) error {
	if s.apiKey == "" {
		return fmt.Errorf("stt/openai: API key not set")
	}

	// 用 models 端点验证 API Key 有效性
	url := s.baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("stt/openai: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("stt/openai: connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("stt/openai: invalid API key")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("stt/openai: unexpected status: %d", resp.StatusCode)
	}
	return nil
}

// mimeTypeToExt 将 MIME 类型转换为文件扩展名
func mimeTypeToExt(mimeType string) string {
	switch mimeType {
	case "audio/opus", "audio/ogg":
		return ".ogg"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	case "audio/flac":
		return ".flac"
	case "audio/m4a", "audio/mp4":
		return ".m4a"
	case "audio/webm":
		return ".webm"
	default:
		return ".ogg" // 飞书语音默认 opus → ogg
	}
}

// truncateString 截断字符串到指定长度
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
