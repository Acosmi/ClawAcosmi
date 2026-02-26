package understanding

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

// TS 对照: media-understanding/providers/openai.ts (~150L)
// OpenAI Provider — 音频转录 (Whisper) + 图像描述 (GPT-4V)

// NewOpenAIProvider 创建 OpenAI Provider。
func NewOpenAIProvider() *Provider {
	return &Provider{
		ID: "openai",
		Capabilities: []Capability{
			{Kind: KindAudioTranscription, Models: []string{"whisper-1"}},
			{Kind: KindImageDescription, Models: []string{"gpt-4o-mini", "gpt-4o", "gpt-4-turbo"}},
		},
		TranscribeAudio: openaiTranscribeAudio,
		DescribeImage:   openaiDescribeImage,
	}
}

// openaiTranscribeAudio OpenAI 音频转录（Whisper API）。
// POST multipart/form-data to https://api.openai.com/v1/audio/transcriptions
// TS 对照: providers/openai.ts transcribeAudio
func openaiTranscribeAudio(req AudioTranscriptionRequest) (*AudioTranscriptionResult, error) {
	if req.Attachment.Path == "" && req.Attachment.URL == "" {
		return nil, fmt.Errorf("openai: 音频转录需要文件路径或 URL")
	}
	model := req.Model
	if model == "" {
		model = DefaultAudioModels["openai"]
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("openai: 缺少 OPENAI_API_KEY 环境变量")
	}

	// 读取音频文件
	audioData, err := readAttachmentData(req.Attachment)
	if err != nil {
		return nil, fmt.Errorf("openai: 读取音频文件失败: %w", err)
	}

	// 构建 multipart 请求
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", model)
	if req.Language != "" {
		_ = writer.WriteField("language", req.Language)
	}
	if req.Prompt != "" {
		_ = writer.WriteField("prompt", req.Prompt)
	}
	_ = writer.WriteField("response_format", "json")

	fileName := "audio.wav"
	if req.Attachment.Path != "" {
		fileName = req.Attachment.Path
	}
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("openai: 创建 form file 失败: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return nil, fmt.Errorf("openai: 写入音频数据失败: %w", err)
	}
	writer.Close()

	// 发送请求
	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	httpReq, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/transcriptions", &body)
	if err != nil {
		return nil, fmt.Errorf("openai: 创建请求失败: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("openai: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	var result struct {
		Text     string `json:"text"`
		Language string `json:"language"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("openai: 解析响应失败: %w", err)
	}

	text := result.Text
	if req.MaxChars > 0 && len([]rune(text)) > req.MaxChars {
		text = string([]rune(text)[:req.MaxChars])
	}

	return &AudioTranscriptionResult{
		Text:     text,
		Language: result.Language,
	}, nil
}

// openaiDescribeImage OpenAI 图像描述（GPT-4V API）。
// POST /v1/chat/completions with image content
// TS 对照: providers/openai.ts describeImage
func openaiDescribeImage(req ImageDescriptionRequest) (*ImageDescriptionResult, error) {
	if req.Attachment.Path == "" && req.Attachment.URL == "" {
		return nil, fmt.Errorf("openai: 图像描述需要文件路径或 URL")
	}
	model := req.Model
	if model == "" {
		model = DefaultImageModels["openai"]
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("openai: 缺少 OPENAI_API_KEY 环境变量")
	}

	prompt := req.Prompt
	if prompt == "" {
		prompt = "Describe this image in detail."
	}

	// 构建图像内容
	var imageContent map[string]interface{}
	if req.Attachment.URL != "" {
		imageContent = map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]string{
				"url": req.Attachment.URL,
			},
		}
	} else {
		// 本地文件 → base64
		data, err := os.ReadFile(req.Attachment.Path)
		if err != nil {
			return nil, fmt.Errorf("openai: 读取图像文件失败: %w", err)
		}
		mime := req.Attachment.MIME
		if mime == "" {
			mime = "image/jpeg"
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		imageContent = map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]string{
				"url": fmt.Sprintf("data:%s;base64,%s", mime, b64),
			},
		}
	}

	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []interface{}{
					map[string]string{"type": "text", "text": prompt},
					imageContent,
				},
			},
		},
		"max_tokens": 1024,
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("openai: 序列化请求失败: %w", err)
	}

	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	httpReq, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("openai: 创建请求失败: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("openai: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("openai: 解析响应失败: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("openai: 响应无内容")
	}

	text := chatResp.Choices[0].Message.Content
	if req.MaxChars > 0 && len([]rune(text)) > req.MaxChars {
		text = string([]rune(text)[:req.MaxChars])
	}

	return &ImageDescriptionResult{Text: text}, nil
}

// ---------- 辅助函数 ----------

// readAttachmentData 读取媒体附件数据。
func readAttachmentData(att MediaAttachment) ([]byte, error) {
	if att.Path != "" {
		return os.ReadFile(att.Path)
	}
	if att.URL != "" {
		resp, err := http.Get(att.URL) //nolint:gosec // 内部调用，URL 已通过上层验证
		if err != nil {
			return nil, fmt.Errorf("下载附件失败: %w", err)
		}
		defer resp.Body.Close()
		return io.ReadAll(io.LimitReader(resp.Body, 100*1024*1024)) // 100MB limit
	}
	return nil, fmt.Errorf("附件无路径或 URL")
}
