package transcript

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ---------- 缓存追踪 ----------

// TS 参考: src/agents/cache-trace.ts (295 行)

// CacheTraceStage 缓存追踪阶段。
type CacheTraceStage string

const (
	StageResolveModel CacheTraceStage = "resolve_model"
	StagePreMessages  CacheTraceStage = "pre_messages"
	StageCacheHit     CacheTraceStage = "cache_hit"
	StageCacheMiss    CacheTraceStage = "cache_miss"
	StagePostStream   CacheTraceStage = "post_stream"
	StageError        CacheTraceStage = "error"
	StageRecovery     CacheTraceStage = "recovery"
)

// CacheTraceEvent 缓存追踪事件。
type CacheTraceEvent struct {
	Ts            string            `json:"ts"`
	Seq           int               `json:"seq"`
	Stage         CacheTraceStage   `json:"stage"`
	RunID         string            `json:"runId,omitempty"`
	SessionID     string            `json:"sessionId,omitempty"`
	SessionKey    string            `json:"sessionKey,omitempty"`
	Provider      string            `json:"provider,omitempty"`
	ModelID       string            `json:"modelId,omitempty"`
	ModelAPI      string            `json:"modelApi,omitempty"`
	WorkspaceDir  string            `json:"workspaceDir,omitempty"`
	MessageCount  int               `json:"messageCount,omitempty"`
	MessageDigest string            `json:"messagesDigest,omitempty"`
	SystemDigest  string            `json:"systemDigest,omitempty"`
	Note          string            `json:"note,omitempty"`
	Error         string            `json:"error,omitempty"`
	Extra         map[string]string `json:"extra,omitempty"`
}

// CacheTraceConfig 缓存追踪配置。
type CacheTraceConfig struct {
	Enabled         bool   `json:"enabled"`
	FilePath        string `json:"filePath"`
	IncludeMessages bool   `json:"includeMessages"`
	IncludePrompt   bool   `json:"includePrompt"`
	IncludeSystem   bool   `json:"includeSystem"`
}

// CacheTrace 缓存追踪器。
type CacheTrace struct {
	Config  CacheTraceConfig
	mu      sync.Mutex
	seq     int
	baseEvt CacheTraceEvent
	writer  *traceWriter
}

type traceWriter struct {
	mu       sync.Mutex
	filePath string
	file     *os.File
}

var (
	writersMu sync.Mutex
	writers   = make(map[string]*traceWriter)
)

// ResolveCacheTraceConfig 解析缓存追踪配置。
func ResolveCacheTraceConfig(stateDir, envTrace string) CacheTraceConfig {
	enabled := strings.EqualFold(envTrace, "true") || envTrace == "1"
	if !enabled {
		return CacheTraceConfig{Enabled: false}
	}

	filePath := filepath.Join(stateDir, "cache-trace.ndjson")
	return CacheTraceConfig{
		Enabled:         true,
		FilePath:        filePath,
		IncludeMessages: false,
		IncludePrompt:   false,
		IncludeSystem:   false,
	}
}

// CreateCacheTrace 创建缓存追踪器。
func CreateCacheTrace(config CacheTraceConfig, runID, sessionID, sessionKey, provider, modelID, modelAPI, workspaceDir string) *CacheTrace {
	if !config.Enabled {
		return nil
	}

	return &CacheTrace{
		Config: config,
		baseEvt: CacheTraceEvent{
			RunID:        runID,
			SessionID:    sessionID,
			SessionKey:   sessionKey,
			Provider:     provider,
			ModelID:      modelID,
			ModelAPI:     modelAPI,
			WorkspaceDir: workspaceDir,
		},
		writer: getWriter(config.FilePath),
	}
}

// RecordStage 记录追踪阶段。
func (ct *CacheTrace) RecordStage(stage CacheTraceStage, note string) {
	if ct == nil {
		return
	}
	ct.mu.Lock()
	ct.seq++
	seq := ct.seq
	ct.mu.Unlock()

	evt := ct.baseEvt
	evt.Ts = time.Now().UTC().Format(time.RFC3339Nano)
	evt.Seq = seq
	evt.Stage = stage
	evt.Note = note

	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	ct.writer.write(string(data))
}

// Digest 计算内容摘要。
func Digest(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:8])
}

// SummarizeMessages 摘要消息。
func SummarizeMessages(messages []map[string]interface{}) (count int, digest string) {
	count = len(messages)
	if count == 0 {
		return 0, ""
	}
	data, err := json.Marshal(messages)
	if err != nil {
		return count, ""
	}
	digest = Digest(string(data))
	return
}

func getWriter(filePath string) *traceWriter {
	writersMu.Lock()
	defer writersMu.Unlock()

	if w, ok := writers[filePath]; ok {
		return w
	}

	w := &traceWriter{filePath: filePath}
	writers[filePath] = w
	return w
}

func (w *traceWriter) write(line string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		dir := filepath.Dir(w.filePath)
		os.MkdirAll(dir, 0755)
		f, err := os.OpenFile(w.filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cache-trace: failed to open %s: %v\n", w.filePath, err)
			return
		}
		w.file = f
	}

	w.file.WriteString(line + "\n")
}
