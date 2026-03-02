package media

// stt_dashscope.go — 阿里云 DashScope 原生 STT 实现（修复 sensevoice-v1 不兼容 OpenAI API 的问题）
//
// DashScope 的 OpenAI 兼容模式 **不覆盖** /audio/transcriptions 端点，
// 因此不能用 OpenAISTT 通过兼容路径调用 sensevoice-v1。
//
// 本文件使用 DashScope 录音文件识别原生 REST API：
//   1. POST /api/v1/services/audio/asr/transcription  → 提交异步转录任务
//   2. GET  /api/v1/tasks/{task_id}                    → 轮询任务状态
//
// 音频通过 data: URL 内联传入（DashScope 支持 data: 前缀的 base64 输入），
// 无需先上传到 OSS 或公网存储。

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// DashScopeSTT 阿里云 DashScope 原生 STT Provider
type DashScopeSTT struct {
	apiKey  string
	model   string
	baseURL string
	lang    string
}

// NewDashScopeSTT 创建 DashScope STT Provider
func NewDashScopeSTT(cfg *types.STTConfig) *DashScopeSTT {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com"
	}
	// 去除尾部斜杠和兼容路径后缀
	baseURL = strings.TrimSuffix(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/compatible-mode/v1")
	baseURL = strings.TrimSuffix(baseURL, "/api/v1")

	model := cfg.Model
	if model == "" {
		model = "sensevoice-v1"
	}
	return &DashScopeSTT{
		apiKey:  cfg.APIKey,
		model:   model,
		baseURL: baseURL,
		lang:    cfg.Language,
	}
}

// Name 返回 Provider 名称
func (d *DashScopeSTT) Name() string {
	return "dashscope"
}

// ---------- 内部 API 类型 ----------

// dashScopeSubmitRequest 提交转录任务请求体
type dashScopeSubmitRequest struct {
	Model  string                 `json:"model"`
	Input  dashScopeSubmitInput   `json:"input"`
	Params map[string]interface{} `json:"parameters,omitempty"`
}

type dashScopeSubmitInput struct {
	FileURLs []string `json:"file_urls"`
}

// dashScopeSubmitResponse 提交任务响应
type dashScopeSubmitResponse struct {
	RequestID string `json:"request_id"`
	Output    struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
	} `json:"output"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// dashScopeTaskResponse 查询任务响应
type dashScopeTaskResponse struct {
	RequestID string `json:"request_id"`
	Output    struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"` // PENDING / RUNNING / SUCCEEDED / FAILED
		Results    []struct {
			FileURL          string `json:"file_url"`
			TranscriptionURL string `json:"transcription_url,omitempty"`
			// 内联结果（部分模型直接返回）
			SubtaskStatus string `json:"subtask_status,omitempty"`
		} `json:"results,omitempty"`
	} `json:"output"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// dashScopeTranscriptionResult 转录结果 JSON（从 transcription_url 下载）
type dashScopeTranscriptionResult struct {
	Transcripts []struct {
		ChannelID       int    `json:"channel_id"`
		ContentDuration int    `json:"content_duration_in_milliseconds"`
		Text            string `json:"text"`
		Sentences       []struct {
			BeginTime int    `json:"begin_time"`
			EndTime   int    `json:"end_time"`
			Text      string `json:"text"`
		} `json:"sentences,omitempty"`
	} `json:"transcripts"`
}

// ---------- STTProvider 接口实现 ----------

// Transcribe 将音频数据转录为文本
func (d *DashScopeSTT) Transcribe(ctx context.Context, audioData []byte, mimeType string) (string, error) {
	if len(audioData) == 0 {
		return "", fmt.Errorf("stt/dashscope: empty audio data")
	}
	if d.apiKey == "" {
		return "", fmt.Errorf("stt/dashscope: API key not set")
	}

	// 将音频编码为 data: URL（DashScope 支持 data: 前缀的输入）
	if mimeType == "" {
		mimeType = "audio/webm"
	}
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(audioData))

	// 1. 提交异步转录任务
	taskID, err := d.submitTask(ctx, dataURL)
	if err != nil {
		return "", fmt.Errorf("stt/dashscope: submit task: %w", err)
	}

	slog.Info("stt/dashscope: task submitted",
		"task_id", taskID,
		"model", d.model,
		"audio_size", len(audioData),
	)

	// 2. 轮询任务状态（最多 60 秒）
	text, err := d.pollTask(ctx, taskID)
	if err != nil {
		return "", fmt.Errorf("stt/dashscope: poll task: %w", err)
	}

	slog.Info("stt/dashscope: transcription complete",
		"task_id", taskID,
		"text_len", len(text),
	)
	return text, nil
}

// TestConnection 测试 API 连接
func (d *DashScopeSTT) TestConnection(ctx context.Context) error {
	if d.apiKey == "" {
		return fmt.Errorf("stt/dashscope: API key not set")
	}

	// 调用 DashScope models 列表验证 API Key
	url := d.baseURL + "/api/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("stt/dashscope: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("stt/dashscope: connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("stt/dashscope: invalid API key (status %d)", resp.StatusCode)
	}
	// 即使 models 端点返回非 200（部分 DashScope 环境无此端点），
	// 只要不是 401/403 就视为连接测试通过
	return nil
}

// ---------- 内部方法 ----------

// submitTask 提交异步转录任务
func (d *DashScopeSTT) submitTask(ctx context.Context, dataURL string) (string, error) {
	payload := dashScopeSubmitRequest{
		Model: d.model,
		Input: dashScopeSubmitInput{
			FileURLs: []string{dataURL},
		},
	}

	// 语言参数
	if d.lang != "" {
		payload.Params = map[string]interface{}{
			"language_hints": []string{d.lang},
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := d.baseURL + "/api/v1/services/audio/asr/transcription"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
	req.Header.Set("Content-Type", "application/json")
	// 启用异步模式
	req.Header.Set("X-DashScope-Async", "enable")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: status=%d body=%s",
			resp.StatusCode, truncateString(string(respBody), 500))
	}

	var result dashScopeSubmitResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Code != "" {
		return "", fmt.Errorf("API error: code=%s message=%s", result.Code, result.Message)
	}

	if result.Output.TaskID == "" {
		return "", fmt.Errorf("empty task_id in response")
	}

	return result.Output.TaskID, nil
}

// pollTask 轮询任务状态直到完成
func (d *DashScopeSTT) pollTask(ctx context.Context, taskID string) (string, error) {
	url := d.baseURL + "/api/v1/tasks/" + taskID

	// 轮询间隔: 500ms → 1s → 2s → 2s → ...
	intervals := []time.Duration{
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
	}

	// 最大轮询次数: 约 4 分钟 (500ms + 1s + 2s*118)
	const maxPollAttempts = 120

	for attempt := 0; attempt < maxPollAttempts; attempt++ {
		// 选择轮询间隔
		interval := intervals[len(intervals)-1]
		if attempt < len(intervals) {
			interval = intervals[attempt]
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+d.apiKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("read response: %w", err)
		}

		var taskResp dashScopeTaskResponse
		if err := json.Unmarshal(respBody, &taskResp); err != nil {
			return "", fmt.Errorf("parse task response: %w", err)
		}

		switch taskResp.Output.TaskStatus {
		case "SUCCEEDED":
			return d.extractText(ctx, taskResp)
		case "FAILED":
			msg := taskResp.Message
			if msg == "" {
				msg = "transcription failed"
			}
			return "", fmt.Errorf("task failed: %s", msg)
		case "PENDING", "RUNNING":
			// 继续轮询
			slog.Debug("stt/dashscope: polling",
				"task_id", taskID,
				"status", taskResp.Output.TaskStatus,
				"attempt", attempt,
			)
		default:
			return "", fmt.Errorf("unknown task status: %s", taskResp.Output.TaskStatus)
		}
	}

	return "", fmt.Errorf("stt/dashscope: polling timeout after %d attempts for task %s", maxPollAttempts, taskID)
}

// extractText 从任务结果中提取转录文本
func (d *DashScopeSTT) extractText(ctx context.Context, taskResp dashScopeTaskResponse) (string, error) {
	if len(taskResp.Output.Results) == 0 {
		return "", fmt.Errorf("no results in task response")
	}

	// 获取 transcription_url 并下载结果
	transcriptionURL := taskResp.Output.Results[0].TranscriptionURL
	if transcriptionURL == "" {
		return "", fmt.Errorf("no transcription_url in result")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, transcriptionURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download transcription: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read transcription: %w", err)
	}

	var result dashScopeTranscriptionResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse transcription: %w", err)
	}

	// 合并所有 transcript 文本
	var texts []string
	for _, t := range result.Transcripts {
		if t.Text != "" {
			texts = append(texts, t.Text)
		}
	}

	if len(texts) == 0 {
		return "", nil // 空音频或无法识别
	}
	return strings.Join(texts, " "), nil
}
