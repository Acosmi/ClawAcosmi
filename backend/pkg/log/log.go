// Package log 提供结构化日志系统。
//
// 基于 Go 标准库 log/slog 封装，支持子系统分级日志和 JSON 输出。
// 继承自原版 TypeScript 的 createSubsystemLogger 模式。
//
// 功能:
//   - 子系统标签 (subsystem)
//   - 动态日志级别控制
//   - 同时输出到 stderr (控制台) 和日志文件
//   - 文件日志按日期自动滚动
package log

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// 包级全局状态（对应原版 state.ts）
var (
	globalMu         sync.RWMutex
	globalLevel      types.LogLevel = types.LogInfo
	globalFileWriter *FileWriter
)

// SetGlobalLevel 设置全局最低日志级别。
// 低于此级别的日志将被过滤（控制台和文件均适用）。
func SetGlobalLevel(level types.LogLevel) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLevel = NormalizeLogLevel(string(level), types.LogInfo)
}

// GetGlobalLevel 获取当前全局日志级别。
func GetGlobalLevel() types.LogLevel {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalLevel
}

// EnableFileLogging 启用文件日志输出。
// dir 为日志目录，为空则使用 DefaultLogDir。
// 调用后，标准 slog 和自定义 Logger 的输出都将同时写入文件。
func EnableFileLogging(dir string) {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalFileWriter != nil {
		_ = globalFileWriter.Close()
	}
	globalFileWriter = NewFileWriter(dir)

	// 将 Go 默认 slog 也重定向到文件+控制台，
	// 这样 gateway 中 slog.Info(...) 等调用也能写入滚动日志文件。
	stderrHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug - 4,
	})
	fw := globalFileWriter
	fileHandler := &slogFileHandler{fw: fw}
	slog.SetDefault(slog.New(&teeHandler{primary: stderrHandler, secondary: fileHandler}))
}

// getFileWriter 获取全局文件写入器（可能为 nil）。
func getFileWriter() *FileWriter {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalFileWriter
}

// toSlogLevel 将 types.LogLevel 转换为 slog.Level。
func toSlogLevel(level types.LogLevel) slog.Level {
	switch level {
	case types.LogTrace:
		return slog.LevelDebug - 4 // slog 没有 trace，用更低值
	case types.LogDebug:
		return slog.LevelDebug
	case types.LogInfo:
		return slog.LevelInfo
	case types.LogWarn:
		return slog.LevelWarn
	case types.LogError, types.LogFatal:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Logger 结构化日志器
type Logger struct {
	inner     *slog.Logger
	subsystem string
}

// New 创建一个带子系统标签的日志器。
// 对应原版 createSubsystemLogger(subsystem)。
func New(subsystem string) *Logger {
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug - 4, // 允许所有级别通过 slog，由我们的级别控制过滤
	})

	return &Logger{
		inner:     slog.New(handler).With("subsystem", subsystem),
		subsystem: subsystem,
	}
}

// Child 创建子日志器（继承原版 log.child("xxx") 模式）
func (l *Logger) Child(name string) *Logger {
	childSubsystem := l.subsystem + "/" + name
	return &Logger{
		inner:     l.inner.With("subsystem", childSubsystem),
		subsystem: childSubsystem,
	}
}

// Subsystem 返回日志器的子系统名称。
func (l *Logger) Subsystem() string {
	return l.subsystem
}

// Trace 记录跟踪级别日志
func (l *Logger) Trace(msg string, args ...any) {
	l.logAt(types.LogTrace, msg, args...)
}

// Debug 记录调试级别日志
func (l *Logger) Debug(msg string, args ...any) {
	l.logAt(types.LogDebug, msg, args...)
}

// Info 记录信息级别日志
func (l *Logger) Info(msg string, args ...any) {
	l.logAt(types.LogInfo, msg, args...)
}

// Warn 记录警告级别日志
func (l *Logger) Warn(msg string, args ...any) {
	l.logAt(types.LogWarn, msg, args...)
}

// Error 记录错误级别日志
func (l *Logger) Error(msg string, args ...any) {
	l.logAt(types.LogError, msg, args...)
}

// Fatal 记录致命级别日志
func (l *Logger) Fatal(msg string, args ...any) {
	l.logAt(types.LogFatal, msg, args...)
}

// With 附加键值对上下文
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		inner:     l.inner.With(args...),
		subsystem: l.subsystem,
	}
}

// logAt 核心日志方法 — 检查级别后同时输出到控制台和文件。
func (l *Logger) logAt(level types.LogLevel, msg string, args ...any) {
	currentLevel := GetGlobalLevel()
	if !IsLevelEnabled(level, currentLevel) {
		return
	}

	// 控制台输出 (via slog)
	slogLevel := toSlogLevel(level)
	l.inner.Log(context.TODO(), slogLevel, msg, args...)

	// 文件输出
	// 格式对齐前端 parseLogLine 期望:
	//   { time, message, _meta: { logLevelName, name, date } }
	if fw := getFileWriter(); fw != nil {
		entry := map[string]interface{}{
			"message": msg,
			"_meta": map[string]interface{}{
				"logLevelName": string(level),
				"name":         l.subsystem,
			},
		}
		// 将 args 作为键值对添加
		for i := 0; i+1 < len(args); i += 2 {
			if key, ok := args[i].(string); ok {
				entry[key] = args[i+1]
			}
		}
		_ = fw.WriteEntry(entry) // 日志写入失败不阻塞
	}
}

// ---------- slog tee handler ----------
// 将 Go 标准 slog 输出同时发送到控制台和滚动日志文件。

// teeHandler 将日志同时发送到两个 handler。
type teeHandler struct {
	primary   slog.Handler
	secondary slog.Handler
}

func (h *teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.primary.Enabled(ctx, level) || h.secondary.Enabled(ctx, level)
}

func (h *teeHandler) Handle(ctx context.Context, r slog.Record) error {
	_ = h.primary.Handle(ctx, r.Clone())
	_ = h.secondary.Handle(ctx, r.Clone())
	return nil
}

func (h *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &teeHandler{
		primary:   h.primary.WithAttrs(attrs),
		secondary: h.secondary.WithAttrs(attrs),
	}
}

func (h *teeHandler) WithGroup(name string) slog.Handler {
	return &teeHandler{
		primary:   h.primary.WithGroup(name),
		secondary: h.secondary.WithGroup(name),
	}
}

// slogFileHandler 将 slog.Record 写入 FileWriter（JSON Lines 格式，兼容前端 parseLogLine）。
type slogFileHandler struct {
	fw    *FileWriter
	attrs []slog.Attr
	group string
}

func (h *slogFileHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *slogFileHandler) Handle(_ context.Context, r slog.Record) error {
	levelName := slogLevelToName(r.Level)
	entry := map[string]interface{}{
		"message": r.Message,
		"_meta": map[string]interface{}{
			"logLevelName": levelName,
			"name":         h.group,
		},
	}
	// 收集 attrs
	for _, a := range h.attrs {
		entry[a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		entry[a.Key] = a.Value.Any()
		return true
	})
	return h.fw.WriteEntry(entry)
}

func (h *slogFileHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &slogFileHandler{fw: h.fw, attrs: newAttrs, group: h.group}
}

func (h *slogFileHandler) WithGroup(name string) slog.Handler {
	g := name
	if h.group != "" {
		g = h.group + "/" + name
	}
	return &slogFileHandler{fw: h.fw, attrs: h.attrs, group: g}
}

func slogLevelToName(l slog.Level) string {
	switch {
	case l < slog.LevelDebug:
		return "TRACE"
	case l < slog.LevelInfo:
		return "DEBUG"
	case l < slog.LevelWarn:
		return "INFO"
	case l < slog.LevelError:
		return "WARN"
	default:
		return "ERROR"
	}
}

// jsonMarshalString 安全序列化为 JSON 字符串。
func jsonMarshalString(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
