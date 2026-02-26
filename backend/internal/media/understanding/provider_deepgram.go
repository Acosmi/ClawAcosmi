package understanding

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// TS 对照: media-understanding/providers/deepgram.ts (~80L)
// Deepgram Provider — 音频转录 (Nova-2)

// NewDeepgramProvider 创建 Deepgram Provider。
func NewDeepgramProvider() *Provider {
	return &Provider{
		ID: "deepgram",
		Capabilities: []Capability{
			{Kind: KindAudioTranscription, Models: []string{"nova-2", "nova-2-general"}},
		},
		TranscribeAudio: deepgramTranscribeAudio,
	}
}

// deepgramTranscribeAudio Deepgram 音频转录。
// POST /v1/listen (raw audio body, Authorization: Token)
func deepgramTranscribeAudio(req AudioTranscriptionRequest) (*AudioTranscriptionResult, error) {
	model := req.Model
	if model == "" {
		model = DefaultAudioModels["deepgram"]
	}

	apiKey := os.Getenv("DEEPGRAM_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("deepgram: 缺少 DEEPGRAM_API_KEY 环境变量")
	}

	data, err := readAttachmentData(req.Attachment)
	if err != nil {
		return nil, fmt.Errorf("deepgram: 读取音频失败: %w", err)
	}

	mime := req.Attachment.MIME
	if mime == "" {
		mime = "audio/wav"
	}

	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	apiURL := fmt.Sprintf("https://api.deepgram.com/v1/listen?model=%s&smart_format=true", model)
	if req.Language != "" {
		apiURL += "&language=" + req.Language
	}

	httpReq, err := http.NewRequest("POST", apiURL, io.NopCloser(
		io.LimitReader(bytesReader(data), int64(len(data))),
	))
	if err != nil {
		return nil, fmt.Errorf("deepgram: 创建请求失败: %w", err)
	}
	httpReq.Header.Set("Authorization", "Token "+apiKey)
	httpReq.Header.Set("Content-Type", mime)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("deepgram: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("deepgram: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	var dgResp struct {
		Results struct {
			Channels []struct {
				Alternatives []struct {
					Transcript string `json:"transcript"`
				} `json:"alternatives"`
			} `json:"channels"`
		} `json:"results"`
		Metadata struct {
			Duration float64 `json:"duration"`
		} `json:"metadata"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&dgResp); err != nil {
		return nil, fmt.Errorf("deepgram: 解析响应失败: %w", err)
	}

	text := ""
	if len(dgResp.Results.Channels) > 0 && len(dgResp.Results.Channels[0].Alternatives) > 0 {
		text = dgResp.Results.Channels[0].Alternatives[0].Transcript
	}

	if req.MaxChars > 0 && len([]rune(text)) > req.MaxChars {
		text = string([]rune(text)[:req.MaxChars])
	}

	return &AudioTranscriptionResult{
		Text:       text,
		Language:   req.Language,
		DurationMs: int(dgResp.Metadata.Duration * 1000),
	}, nil
}

// bytesReader 创建 bytes.Reader 作为 io.Reader。
func bytesReader(data []byte) io.Reader {
	return io.NopCloser(
		io.LimitReader(
			func() io.Reader {
				r := &simpleReader{data: data}
				return r
			}(),
			int64(len(data)),
		),
	)
}

// simpleReader 简单的 bytes reader。
type simpleReader struct {
	data []byte
	pos  int
}

func (r *simpleReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
