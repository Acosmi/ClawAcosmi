package runner

// ============================================================================
// Raw Stream 调试日志
// TS 对照: pi-embedded-subscribe.raw-stream.ts → appendRawStream()
//
// 通过环境变量 OPENACOSMI_RAW_STREAM=1 启用，
// 将流式事件以 JSONL 格式追加到日志文件。
// ============================================================================

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// rawStreamOnce 懒初始化：环境变量检查 + 目录创建。
var (
	rawStreamEnabled bool
	rawStreamPath    string
	rawStreamOnce    sync.Once
)

// initRawStream 初始化 raw stream 配置（仅执行一次）。
func initRawStream() {
	rawStreamOnce.Do(func() {
		if os.Getenv("OPENACOSMI_RAW_STREAM") == "" {
			return
		}
		rawStreamEnabled = true

		rawStreamPath = os.Getenv("OPENACOSMI_RAW_STREAM_PATH")
		if rawStreamPath == "" {
			rawStreamPath = filepath.Join("logs", "raw-stream.jsonl")
		}

		// 确保目录存在
		dir := filepath.Dir(rawStreamPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			slog.Warn("raw stream: 创建日志目录失败", "dir", dir, "error", err)
			rawStreamEnabled = false
			return
		}

		slog.Debug("raw stream 已启用", "path", rawStreamPath)
	})
}

// AppendRawStream 将 payload 追加到 raw stream JSONL 日志。
// 非阻塞，所有错误仅记录日志不返回。
// TS 对照: pi-embedded-subscribe.raw-stream.ts → appendRawStream()
func AppendRawStream(payload map[string]interface{}) {
	initRawStream()

	if !rawStreamEnabled {
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		slog.Debug("raw stream: 序列化失败", "error", err)
		return
	}

	f, err := os.OpenFile(rawStreamPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		slog.Debug("raw stream: 打开文件失败", "path", rawStreamPath, "error", err)
		return
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		slog.Debug("raw stream: 写入失败", "error", err)
	}
}

// ResetRawStreamForTest 重置 raw stream 状态（仅用于测试）。
func ResetRawStreamForTest() {
	rawStreamOnce = sync.Once{}
	rawStreamEnabled = false
	rawStreamPath = ""
}
