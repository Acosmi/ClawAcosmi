package tts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// TS 对照: tts/tts.ts L780-1200 (合成引擎)

// ---------- 合成接口 ----------

// SynthesizeParams 合成参数。
type SynthesizeParams struct {
	Text      string
	Config    ResolvedTtsConfig
	Provider  TtsProvider
	OutputFmt OutputFormat
	TimeoutMs int
	Overrides TtsDirectiveOverrides
}

// ---------- 合成函数 ----------

// SynthesizeWithProvider 使用指定 Provider 合成语音。
// TS 对照: tts.ts synthesizeOpenAI / synthesizeElevenLabs / synthesizeEdge
func SynthesizeWithProvider(params SynthesizeParams) (*TtsResult, error) {
	start := time.Now()

	switch params.Provider {
	case ProviderOpenAI:
		return synthesizeOpenAI(params, start)
	case ProviderElevenLabs:
		return synthesizeElevenLabs(params, start)
	case ProviderEdge:
		return synthesizeEdge(params, start)
	default:
		return nil, fmt.Errorf("未知 TTS provider: %s", params.Provider)
	}
}

// ---------- OpenAI 合成 ----------

// synthesizeOpenAI OpenAI TTS 合成。
// TS 对照: tts.ts L780-860
// POST https://api.openai.com/v1/audio/speech
func synthesizeOpenAI(params SynthesizeParams, start time.Time) (*TtsResult, error) {
	apiKey := ResolveTtsApiKey(params.Config, ProviderOpenAI)
	if apiKey == "" {
		return nil, fmt.Errorf("openai: 缺少 API key")
	}

	model := params.Config.OpenAI.Model
	voice := params.Config.OpenAI.Voice
	if params.Overrides.OpenAI != nil {
		if params.Overrides.OpenAI.Model != "" {
			model = params.Overrides.OpenAI.Model
		}
		if params.Overrides.OpenAI.Voice != "" {
			voice = params.Overrides.OpenAI.Voice
		}
	}

	responseFormat := params.OutputFmt.OpenAI
	if responseFormat == "" {
		responseFormat = "mp3"
	}

	// 构建请求体
	reqBody := map[string]interface{}{
		"model":           model,
		"input":           params.Text,
		"voice":           voice,
		"response_format": responseFormat,
	}
	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("openai: 序列化请求失败: %w", err)
	}

	// 发送请求
	timeout := time.Duration(params.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Duration(DefaultTimeoutMs) * time.Millisecond
	}
	client := &http.Client{Timeout: timeout}

	// 使用可配置端点（对齐 TS getOpenAITtsBaseUrl() + /audio/speech）
	baseURL := params.Config.OpenAI.BaseURL
	if baseURL == "" {
		baseURL = DefaultOpenAITtsBaseURL
	}
	apiURL := baseURL + "/audio/speech"

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("openai: 创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return &TtsResult{
			Success:   false,
			Error:     fmt.Sprintf("openai: 请求失败: %v", err),
			LatencyMs: time.Since(start).Milliseconds(),
			Provider:  string(ProviderOpenAI),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return &TtsResult{
			Success:   false,
			Error:     fmt.Sprintf("openai: HTTP %d: %s", resp.StatusCode, string(errBody)),
			LatencyMs: time.Since(start).Milliseconds(),
			Provider:  string(ProviderOpenAI),
		}, nil
	}

	// 写入临时文件
	ext := "." + responseFormat
	audioPath, err := writeAudioToTemp(resp.Body, "openai-tts-*"+ext)
	if err != nil {
		return &TtsResult{
			Success:   false,
			Error:     fmt.Sprintf("openai: 写入音频文件失败: %v", err),
			LatencyMs: time.Since(start).Milliseconds(),
			Provider:  string(ProviderOpenAI),
		}, nil
	}

	return &TtsResult{
		Success:         true,
		AudioPath:       audioPath,
		LatencyMs:       time.Since(start).Milliseconds(),
		Provider:        string(ProviderOpenAI),
		OutputFormat:    responseFormat,
		VoiceCompatible: params.OutputFmt.VoiceCompatible,
	}, nil
}

// ---------- ElevenLabs 合成 ----------

// synthesizeElevenLabs ElevenLabs TTS 合成。
// TS 对照: tts.ts L862-990
// POST {baseUrl}/v1/text-to-speech/{voiceId}
func synthesizeElevenLabs(params SynthesizeParams, start time.Time) (*TtsResult, error) {
	apiKey := ResolveTtsApiKey(params.Config, ProviderElevenLabs)
	if apiKey == "" {
		return nil, fmt.Errorf("elevenlabs: 缺少 API key")
	}

	voiceID := params.Config.ElevenLabs.VoiceID
	modelID := params.Config.ElevenLabs.ModelID
	voiceSettings := params.Config.ElevenLabs.VoiceSettings
	if params.Overrides.ElevenLabs != nil {
		if params.Overrides.ElevenLabs.VoiceID != "" {
			voiceID = params.Overrides.ElevenLabs.VoiceID
		}
		if params.Overrides.ElevenLabs.ModelID != "" {
			modelID = params.Overrides.ElevenLabs.ModelID
		}
		if params.Overrides.ElevenLabs.VoiceSettings != nil {
			voiceSettings = *params.Overrides.ElevenLabs.VoiceSettings
		}
	}

	baseURL := params.Config.ElevenLabs.BaseURL
	if baseURL == "" {
		baseURL = DefaultElevenLabsBaseURL
	}

	outputFormat := params.OutputFmt.ElevenLabs
	if outputFormat == "" {
		outputFormat = "mp3_44100_128"
	}

	// 构建请求体
	reqBody := map[string]interface{}{
		"text":     params.Text,
		"model_id": modelID,
		"voice_settings": map[string]interface{}{
			"stability":         voiceSettings.Stability,
			"similarity_boost":  voiceSettings.SimilarityBoost,
			"style":             voiceSettings.Style,
			"use_speaker_boost": voiceSettings.UseSpeakerBoost,
		},
	}

	// 可选字段
	if params.Config.ElevenLabs.Seed != nil {
		reqBody["seed"] = *params.Config.ElevenLabs.Seed
	}
	if params.Config.ElevenLabs.ApplyTextNormalization != "" {
		reqBody["apply_text_normalization"] = params.Config.ElevenLabs.ApplyTextNormalization
	}
	if params.Config.ElevenLabs.LanguageCode != "" {
		reqBody["language_code"] = params.Config.ElevenLabs.LanguageCode
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs: 序列化请求失败: %w", err)
	}

	// 发送请求
	timeout := time.Duration(params.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Duration(DefaultTimeoutMs) * time.Millisecond
	}
	client := &http.Client{Timeout: timeout}

	apiURL := fmt.Sprintf("%s/v1/text-to-speech/%s?output_format=%s", baseURL, voiceID, outputFormat)
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("elevenlabs: 创建请求失败: %w", err)
	}
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return &TtsResult{
			Success:   false,
			Error:     fmt.Sprintf("elevenlabs: 请求失败: %v", err),
			LatencyMs: time.Since(start).Milliseconds(),
			Provider:  string(ProviderElevenLabs),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return &TtsResult{
			Success:   false,
			Error:     fmt.Sprintf("elevenlabs: HTTP %d: %s", resp.StatusCode, string(errBody)),
			LatencyMs: time.Since(start).Milliseconds(),
			Provider:  string(ProviderElevenLabs),
		}, nil
	}

	// 写入临时文件
	ext := extensionFromElevenLabsFormat(outputFormat)
	audioPath, err := writeAudioToTemp(resp.Body, "elevenlabs-tts-*"+ext)
	if err != nil {
		return &TtsResult{
			Success:   false,
			Error:     fmt.Sprintf("elevenlabs: 写入音频文件失败: %v", err),
			LatencyMs: time.Since(start).Milliseconds(),
			Provider:  string(ProviderElevenLabs),
		}, nil
	}

	return &TtsResult{
		Success:         true,
		AudioPath:       audioPath,
		LatencyMs:       time.Since(start).Milliseconds(),
		Provider:        string(ProviderElevenLabs),
		OutputFormat:    outputFormat,
		VoiceCompatible: params.OutputFmt.VoiceCompatible,
	}, nil
}

// ---------- Edge TTS 合成 ----------

// synthesizeEdge Edge TTS 合成。
// 使用 edge-tts CLI (pip install edge-tts)。
// WebSocket 协议实现延迟到后续迭代。
// TS 对照: tts.ts L992-1100
func synthesizeEdge(params SynthesizeParams, start time.Time) (*TtsResult, error) {
	if !params.Config.Edge.Enabled {
		return nil, fmt.Errorf("edge: TTS 未启用")
	}

	voice := params.Config.Edge.Voice
	outputFormat := params.Config.Edge.OutputFormat

	// 创建临时输出文件
	tmpFile, err := os.CreateTemp("", "edge-tts-*.mp3")
	if err != nil {
		return nil, fmt.Errorf("edge: 创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer func() {
		if _, err := os.Stat(tmpPath); err == nil {
			// 文件会在上层清理，这里不删除
		}
	}()

	// 构建命令
	args := []string{
		"--voice", voice,
		"--text", params.Text,
		"-f", tmpPath,
	}
	if params.Config.Edge.Rate != "" {
		args = append(args, "--rate", params.Config.Edge.Rate)
	}
	if params.Config.Edge.Volume != "" {
		args = append(args, "--volume", params.Config.Edge.Volume)
	}
	if params.Config.Edge.Pitch != "" {
		args = append(args, "--pitch", params.Config.Edge.Pitch)
	}
	if params.Config.Edge.Proxy != "" {
		args = append(args, "--proxy", params.Config.Edge.Proxy)
	}

	// 执行 edge-tts CLI
	timeout := time.Duration(params.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Duration(DefaultTimeoutMs) * time.Millisecond
	}

	cmd := exec.Command("edge-tts", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// 超时控制
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			return &TtsResult{
				Success:   false,
				Error:     fmt.Sprintf("edge: CLI 执行失败: %v, stderr: %s", err, stderr.String()),
				LatencyMs: time.Since(start).Milliseconds(),
				Provider:  string(ProviderEdge),
			}, nil
		}
	case <-time.After(timeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return &TtsResult{
			Success:   false,
			Error:     "edge: CLI 执行超时",
			LatencyMs: time.Since(start).Milliseconds(),
			Provider:  string(ProviderEdge),
		}, nil
	}

	// 检查输出文件
	info, err := os.Stat(tmpPath)
	if err != nil || info.Size() == 0 {
		return &TtsResult{
			Success:   false,
			Error:     "edge: 生成的音频文件为空",
			LatencyMs: time.Since(start).Milliseconds(),
			Provider:  string(ProviderEdge),
		}, nil
	}

	return &TtsResult{
		Success:         true,
		AudioPath:       tmpPath,
		LatencyMs:       time.Since(start).Milliseconds(),
		Provider:        string(ProviderEdge),
		OutputFormat:    outputFormat,
		VoiceCompatible: false,
	}, nil
}

// ---------- 辅助函数 ----------

// writeAudioToTemp 将音频数据写入临时文件。
func writeAudioToTemp(r io.Reader, pattern string) (string, error) {
	tmpDir := os.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, pattern)
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, r); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("写入音频数据失败: %w", err)
	}

	return tmpFile.Name(), nil
}

// extensionFromElevenLabsFormat 从 ElevenLabs 输出格式推断文件扩展名。
func extensionFromElevenLabsFormat(format string) string {
	switch {
	case len(format) >= 4 && format[:4] == "opus":
		return ".opus"
	case len(format) >= 3 && format[:3] == "mp3":
		return ".mp3"
	case len(format) >= 3 && format[:3] == "pcm":
		return ".pcm"
	case len(format) >= 4 && format[:4] == "ulaw":
		return ".ulaw"
	default:
		return filepath.Ext(format)
	}
}
