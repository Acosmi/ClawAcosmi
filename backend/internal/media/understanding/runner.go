package understanding

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// TS 对照: media-understanding/runner.ts (1305L)
// 媒体理解主运行器 — 核心调度逻辑。

// ---------- 运行参数 ----------

// RunParams 运行能力的参数。
// TS 对照: runner.ts RunParams
type RunParams struct {
	Kind       Kind
	Attachment MediaAttachment
	// 模型配置
	ModelEntries []ModelEntry
	// 解析后的参数
	Prompt      string
	MaxChars    int
	MaxBytes    int
	TimeoutMs   int
	Concurrency int
	// 注册表
	Registry *Registry
}

// RunResult 运行能力的结果。
type RunResult struct {
	Output *Output
	Error  error
}

// ---------- 主运行函数 ----------

// RunCapability 运行指定的媒体理解能力。
// 按模型条目列表依次尝试，使用第一个成功的结果。
// TS 对照: runner.ts L850-950 runCapability
func RunCapability(params RunParams) *RunResult {
	registry := params.Registry
	if registry == nil {
		registry = BuildDefaultRegistry()
	}

	entries := params.ModelEntries
	if len(entries) == 0 {
		entries = ResolveModelEntries(params.Kind, nil, registry)
	}
	if len(entries) == 0 {
		return &RunResult{
			Error: fmt.Errorf("没有可用的 Provider 支持 %s", params.Kind),
		}
	}

	// 按 Provider 顺序尝试
	for _, entry := range entries {
		provider := registry.Get(entry.ProviderID)
		if provider == nil {
			log.Printf("[media-understanding] Provider %q 未注册，跳过", entry.ProviderID)
			continue
		}

		output, err := runWithProvider(params, provider, entry.Model)
		if err != nil {
			log.Printf("[media-understanding] Provider %q 失败: %v", entry.ProviderID, err)
			continue // 尝试下一个 Provider
		}
		return &RunResult{Output: output}
	}

	return &RunResult{
		Error: fmt.Errorf("所有 Provider 均失败 (%s, %d 个条目)",
			params.Kind, len(entries)),
	}
}

// runWithProvider 使用指定 Provider 运行能力。
func runWithProvider(params RunParams, provider *Provider, model string) (*Output, error) {
	start := time.Now()

	switch params.Kind {
	case KindAudioTranscription:
		if provider.TranscribeAudio == nil {
			return nil, fmt.Errorf("provider %s 不支持音频转录", provider.ID)
		}
		result, err := provider.TranscribeAudio(AudioTranscriptionRequest{
			Attachment: params.Attachment,
			Model:      model,
			Prompt:     params.Prompt,
			MaxChars:   params.MaxChars,
			TimeoutMs:  params.TimeoutMs,
		})
		if err != nil {
			return nil, err
		}
		return &Output{
			Kind:       params.Kind,
			Provider:   provider.ID,
			Model:      model,
			Attachment: params.Attachment,
			Text:       result.Text,
			DurationMs: int(time.Since(start).Milliseconds()),
		}, nil

	case KindVideoDescription:
		if provider.DescribeVideo == nil {
			return nil, fmt.Errorf("provider %s 不支持视频描述", provider.ID)
		}
		result, err := provider.DescribeVideo(VideoDescriptionRequest{
			Attachment: params.Attachment,
			Model:      model,
			Prompt:     params.Prompt,
			MaxChars:   params.MaxChars,
			TimeoutMs:  params.TimeoutMs,
		})
		if err != nil {
			return nil, err
		}
		return &Output{
			Kind:       params.Kind,
			Provider:   provider.ID,
			Model:      model,
			Attachment: params.Attachment,
			Text:       result.Text,
			DurationMs: int(time.Since(start).Milliseconds()),
		}, nil

	case KindImageDescription:
		if provider.DescribeImage == nil {
			return nil, fmt.Errorf("provider %s 不支持图像描述", provider.ID)
		}
		result, err := provider.DescribeImage(ImageDescriptionRequest{
			Attachment: params.Attachment,
			Model:      model,
			Prompt:     params.Prompt,
			MaxChars:   params.MaxChars,
			MaxBytes:   params.MaxBytes,
			TimeoutMs:  params.TimeoutMs,
		})
		if err != nil {
			return nil, err
		}
		return &Output{
			Kind:       params.Kind,
			Provider:   provider.ID,
			Model:      model,
			Attachment: params.Attachment,
			Text:       result.Text,
			DurationMs: int(time.Since(start).Milliseconds()),
		}, nil

	default:
		return nil, fmt.Errorf("未知能力种类: %s", params.Kind)
	}
}

// ---------- 批量运行 ----------

// RunBatch 批量运行媒体理解能力（使用并发控制）。
// TS 对照: runner.ts L960-1050
func RunBatch(params []RunParams, concurrency int) []*RunResult {
	if concurrency <= 0 {
		concurrency = 3
	}

	tasks := make([]func() (*RunResult, error), len(params))
	for i, p := range params {
		pp := p // 避免闭包捕获问题
		tasks[i] = func() (*RunResult, error) {
			result := RunCapability(pp)
			return result, nil
		}
	}

	results, _ := RunWithConcurrency(tasks, concurrency)
	return results
}

// ---------- CLI 工具探测 ----------

// BinaryProbeResult CLI 工具探测结果。
type BinaryProbeResult struct {
	Name      string
	Available bool
	Path      string
	Version   string
}

// ProbeBinaries 探测可用的 CLI 工具。
// TS 对照: runner.ts L200-280
func ProbeBinaries() []BinaryProbeResult {
	binaries := []struct {
		Name       string
		VersionArg string // 获取版本的参数
	}{
		{"whisper-cpp", "--version"},
		{"whisper", "--version"},          // 替代名称
		{"sherpa-onnx", "--help"},         // sherpa-onnx 可能只支持 --help
		{"sherpa-onnx-offline", "--help"}, // 替代名称
		{"gemini", "--version"},
	}

	var results []BinaryProbeResult
	for _, bin := range binaries {
		path, err := exec.LookPath(bin.Name)
		if err != nil {
			results = append(results, BinaryProbeResult{
				Name:      bin.Name,
				Available: false,
			})
			continue
		}

		// 尝试获取版本信息
		version := ""
		if bin.VersionArg != "" {
			out, err := exec.Command(path, bin.VersionArg).CombinedOutput()
			if err == nil {
				// 取第一行作为版本信息
				lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
				if len(lines) > 0 {
					version = strings.TrimSpace(lines[0])
				}
			}
		}

		results = append(results, BinaryProbeResult{
			Name:      bin.Name,
			Available: true,
			Path:      path,
			Version:   version,
		})
	}

	return results
}
