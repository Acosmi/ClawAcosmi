package media

// stt_local.go — 本地 whisper.cpp STT 实现（Phase C 新增）
// 调用 whisper.cpp CLI 进行离线语音转文本
// 需要用户预安装 whisper.cpp 并指定路径

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// LocalWhisperSTT 本地 whisper.cpp 实现
type LocalWhisperSTT struct {
	binaryPath string
	modelPath  string
	language   string
}

// NewLocalWhisperSTT 创建本地 Whisper STT Provider
func NewLocalWhisperSTT(cfg *types.STTConfig) *LocalWhisperSTT {
	return &LocalWhisperSTT{
		binaryPath: cfg.BinaryPath,
		modelPath:  cfg.ModelPath,
		language:   cfg.Language,
	}
}

// Name 返回 Provider 名称
func (s *LocalWhisperSTT) Name() string {
	return "local-whisper"
}

// Transcribe 调用 whisper.cpp CLI 转录音频
func (s *LocalWhisperSTT) Transcribe(ctx context.Context, audioData []byte, mimeType string) (string, error) {
	if len(audioData) == 0 {
		return "", fmt.Errorf("stt/local: empty audio data")
	}

	// 写入临时文件
	tmpDir := os.TempDir()
	ext := mimeTypeToExt(mimeType)
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("stt_input_%d%s", os.Getpid(), ext))
	if err := os.WriteFile(tmpFile, audioData, 0644); err != nil {
		return "", fmt.Errorf("stt/local: write temp file: %w", err)
	}
	defer os.Remove(tmpFile)

	// 构建命令
	args := []string{
		"-m", s.modelPath,
		"-f", tmpFile,
		"--output-txt",
		"--no-timestamps",
	}
	if s.language != "" {
		args = append(args, "-l", s.language)
	}

	cmd := exec.CommandContext(ctx, s.binaryPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("stt/local: whisper.cpp failed: %w, output: %s",
			err, truncateString(string(output), 500))
	}

	text := strings.TrimSpace(string(output))
	slog.Info("stt/local: transcription complete",
		"audio_size", len(audioData),
		"text_len", len(text),
	)
	return text, nil
}

// TestConnection 验证 whisper.cpp 可用性
func (s *LocalWhisperSTT) TestConnection(ctx context.Context) error {
	if s.binaryPath == "" {
		return fmt.Errorf("stt/local: binary path not set")
	}
	if s.modelPath == "" {
		return fmt.Errorf("stt/local: model path not set")
	}

	// 检查二进制文件存在
	if _, err := os.Stat(s.binaryPath); os.IsNotExist(err) {
		return fmt.Errorf("stt/local: binary not found: %s", s.binaryPath)
	}

	// 检查模型文件存在
	if _, err := os.Stat(s.modelPath); os.IsNotExist(err) {
		return fmt.Errorf("stt/local: model not found: %s", s.modelPath)
	}

	// 尝试运行 --help 验证可执行
	cmd := exec.CommandContext(ctx, s.binaryPath, "--help")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stt/local: binary not executable: %w", err)
	}

	return nil
}
