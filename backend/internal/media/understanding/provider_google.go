package understanding

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// TS 对照: media-understanding/providers/google.ts (~180L)
// Google Provider — 音频转录 + 视频描述 + 图像描述 (Gemini API)

// NewGoogleProvider 创建 Google Provider。
func NewGoogleProvider() *Provider {
	return &Provider{
		ID: "google",
		Capabilities: []Capability{
			{Kind: KindAudioTranscription, Models: []string{"gemini-2.0-flash", "gemini-1.5-pro"}},
			{Kind: KindVideoDescription, Models: []string{"gemini-2.0-flash", "gemini-1.5-pro"}},
			{Kind: KindImageDescription, Models: []string{"gemini-2.0-flash", "gemini-1.5-pro"}},
		},
		TranscribeAudio: googleTranscribeAudio,
		DescribeVideo:   googleDescribeVideo,
		DescribeImage:   googleDescribeImage,
	}
}

// googleTranscribeAudio Google 音频转录（Gemini API）。
// POST /v1beta/models/{model}:generateContent
func googleTranscribeAudio(req AudioTranscriptionRequest) (*AudioTranscriptionResult, error) {
	model := req.Model
	if model == "" {
		model = DefaultAudioModels["google"]
	}

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("google: 缺少 GOOGLE_API_KEY 环境变量")
	}

	data, err := readAttachmentData(req.Attachment)
	if err != nil {
		return nil, fmt.Errorf("google: 读取音频失败: %w", err)
	}

	mime := req.Attachment.MIME
	if mime == "" {
		mime = "audio/wav"
	}

	prompt := "Transcribe this audio."
	if req.Language != "" {
		prompt = fmt.Sprintf("Transcribe this audio in %s.", req.Language)
	}

	text, err := geminiGenerateContent(apiKey, model, prompt, data, mime, req.TimeoutMs)
	if err != nil {
		return nil, err
	}

	if req.MaxChars > 0 && len([]rune(text)) > req.MaxChars {
		text = string([]rune(text)[:req.MaxChars])
	}

	return &AudioTranscriptionResult{
		Text:     text,
		Language: req.Language,
	}, nil
}

// googleDescribeVideo Google 视频描述（Gemini API）。
func googleDescribeVideo(req VideoDescriptionRequest) (*VideoDescriptionResult, error) {
	model := req.Model
	if model == "" {
		model = DefaultVideoModels["google"]
	}

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("google: 缺少 GOOGLE_API_KEY 环境变量")
	}

	data, err := readAttachmentData(req.Attachment)
	if err != nil {
		return nil, fmt.Errorf("google: 读取视频失败: %w", err)
	}

	mime := req.Attachment.MIME
	if mime == "" {
		mime = "video/mp4"
	}

	prompt := req.Prompt
	if prompt == "" {
		prompt = "Describe this video in detail."
	}

	text, err := geminiGenerateContent(apiKey, model, prompt, data, mime, req.TimeoutMs)
	if err != nil {
		return nil, err
	}

	if req.MaxChars > 0 && len([]rune(text)) > req.MaxChars {
		text = string([]rune(text)[:req.MaxChars])
	}

	return &VideoDescriptionResult{Text: text}, nil
}

// googleDescribeImage Google 图像描述（Gemini API）。
func googleDescribeImage(req ImageDescriptionRequest) (*ImageDescriptionResult, error) {
	model := req.Model
	if model == "" {
		model = DefaultImageModels["google"]
	}

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("google: 缺少 GOOGLE_API_KEY 环境变量")
	}

	data, err := readAttachmentData(req.Attachment)
	if err != nil {
		return nil, fmt.Errorf("google: 读取图像失败: %w", err)
	}

	mime := req.Attachment.MIME
	if mime == "" {
		mime = "image/jpeg"
	}

	prompt := req.Prompt
	if prompt == "" {
		prompt = "Describe this image in detail."
	}

	text, err := geminiGenerateContent(apiKey, model, prompt, data, mime, req.TimeoutMs)
	if err != nil {
		return nil, err
	}

	if req.MaxChars > 0 && len([]rune(text)) > req.MaxChars {
		text = string([]rune(text)[:req.MaxChars])
	}

	return &ImageDescriptionResult{Text: text}, nil
}

// ---------- Gemini 共用函数 ----------

// geminiGenerateContent 调用 Gemini generateContent API。
func geminiGenerateContent(apiKey, model, prompt string, data []byte, mime string, timeoutMs int) (string, error) {
	b64Data := base64.StdEncoding.EncodeToString(data)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []interface{}{
					map[string]string{"text": prompt},
					map[string]interface{}{
						"inline_data": map[string]string{
							"mime_type": mime,
							"data":      b64Data,
						},
					},
				},
			},
		},
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("google: 序列化请求失败: %w", err)
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", fmt.Errorf("google: 创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("google: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("google: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return "", fmt.Errorf("google: 解析响应失败: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("google: 无有效响应内容")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}
