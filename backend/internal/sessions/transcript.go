// Package sessions — 会话转录管理。
//
// 对齐 TS: src/config/sessions/transcript.ts (148L)
package sessions

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ---------- 转录文本推导 ----------

// stripQuery 去除 URL 中的 query 和 hash 部分。
func stripQuery(value string) string {
	noHash := strings.SplitN(value, "#", 2)[0]
	return strings.SplitN(noHash, "?", 2)[0]
}

// extractFileNameFromMediaURL 从媒体 URL 提取文件名。
func extractFileNameFromMediaURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	cleaned := stripQuery(trimmed)
	// 尝试作为 URL 解析
	parsed, err := url.Parse(cleaned)
	if err == nil && parsed.Scheme != "" {
		// 有效的 URL（有 scheme），只从 Path 提取
		base := filepath.Base(parsed.Path)
		if base != "" && base != "/" && base != "." {
			decoded, decErr := url.PathUnescape(base)
			if decErr == nil {
				return decoded
			}
			return base
		}
		return "" // URL 有效但无可提取文件名
	}
	// 回退：非 URL 字符串，直接取 basename
	base := filepath.Base(cleaned)
	if base == "" || base == "/" || base == "." {
		return ""
	}
	return base
}

// ResolveMirroredTranscriptText 解析镜像转录的文本内容。
// 对齐 TS: transcript.ts resolveMirroredTranscriptText()
func ResolveMirroredTranscriptText(text string, mediaURLs []string) string {
	// 过滤有效的媒体 URL
	validURLs := make([]string, 0, len(mediaURLs))
	for _, u := range mediaURLs {
		if strings.TrimSpace(u) != "" {
			validURLs = append(validURLs, u)
		}
	}

	if len(validURLs) > 0 {
		names := make([]string, 0, len(validURLs))
		for _, u := range validURLs {
			name := extractFileNameFromMediaURL(u)
			if strings.TrimSpace(name) != "" {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			return strings.Join(names, ", ")
		}
		return "media"
	}

	trimmed := strings.TrimSpace(text)
	if trimmed != "" {
		return trimmed
	}
	return ""
}

// ---------- 转录文件操作 ----------

// SessionHeader JSONL 转录文件的头记录。
type SessionHeader struct {
	Type      string `json:"type"`
	Version   int    `json:"version"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Cwd       string `json:"cwd,omitempty"`
}

// TranscriptMessage 追加到转录文件的消息记录。
type TranscriptMessage struct {
	Role       string        `json:"role"`
	Content    []ContentPart `json:"content"`
	API        string        `json:"api,omitempty"`
	Provider   string        `json:"provider,omitempty"`
	Model      string        `json:"model,omitempty"`
	Usage      *UsageInfo    `json:"usage,omitempty"`
	StopReason string        `json:"stopReason,omitempty"`
	Timestamp  int64         `json:"timestamp"`
}

// ContentPart 消息内容块。
type ContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// UsageInfo token 使用信息。
type UsageInfo struct {
	Input       int       `json:"input"`
	Output      int       `json:"output"`
	CacheRead   int       `json:"cacheRead"`
	CacheWrite  int       `json:"cacheWrite"`
	TotalTokens int       `json:"totalTokens"`
	Cost        *CostInfo `json:"cost,omitempty"`
}

// CostInfo 费用信息。
type CostInfo struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
	Total      float64 `json:"total"`
}

// CurrentSessionVersion 当前会话版本号。
const CurrentSessionVersion = 3

// ensureSessionHeader 确保转录文件存在并包含头记录。
func ensureSessionHeader(sessionFile, sessionID string) error {
	if _, err := os.Stat(sessionFile); err == nil {
		return nil // 文件已存在
	}
	dir := filepath.Dir(sessionFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建转录目录失败: %w", err)
	}

	cwd, _ := os.Getwd()
	header := SessionHeader{
		Type:      "session",
		Version:   CurrentSessionVersion,
		ID:        sessionID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Cwd:       cwd,
	}
	data, err := json.Marshal(header)
	if err != nil {
		return fmt.Errorf("序列化头记录失败: %w", err)
	}
	return os.WriteFile(sessionFile, append(data, '\n'), 0o644)
}

// AppendAssistantMessageResult 追加消息的结果。
type AppendAssistantMessageResult struct {
	OK          bool
	SessionFile string
	Reason      string
}

// AppendAssistantMessageToSessionTranscript 追加助手消息到会话转录。
// 对齐 TS: transcript.ts appendAssistantMessageToSessionTranscript()
func AppendAssistantMessageToSessionTranscript(params AppendTranscriptParams) AppendAssistantMessageResult {
	sessionKey := strings.TrimSpace(params.SessionKey)
	if sessionKey == "" {
		return AppendAssistantMessageResult{OK: false, Reason: "missing sessionKey"}
	}

	mirrorText := ResolveMirroredTranscriptText(params.Text, params.MediaURLs)
	if mirrorText == "" {
		return AppendAssistantMessageResult{OK: false, Reason: "empty text"}
	}

	storePath := params.StorePath
	if storePath == "" {
		storePath = ResolveDefaultSessionStorePath(params.AgentID)
	}

	store := NewSessionStore(storePath)
	entry, err := store.Get(sessionKey)
	if err != nil || entry == nil || entry.SessionID == "" {
		return AppendAssistantMessageResult{OK: false, Reason: fmt.Sprintf("unknown sessionKey: %s", sessionKey)}
	}

	sessionFile := strings.TrimSpace(entry.SessionFile)
	if sessionFile == "" {
		sessionFile = ResolveSessionTranscriptPath(entry.SessionID, params.AgentID, nil)
	}

	if err := ensureSessionHeader(sessionFile, entry.SessionID); err != nil {
		return AppendAssistantMessageResult{OK: false, Reason: err.Error()}
	}

	msg := TranscriptMessage{
		Role:     "assistant",
		Content:  []ContentPart{{Type: "text", Text: mirrorText}},
		API:      "openai-responses",
		Provider: "openacosmi",
		Model:    "delivery-mirror",
		Usage: &UsageInfo{
			Cost: &CostInfo{},
		},
		StopReason: "stop",
		Timestamp:  time.Now().UnixMilli(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return AppendAssistantMessageResult{OK: false, Reason: err.Error()}
	}

	f, err := os.OpenFile(sessionFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return AppendAssistantMessageResult{OK: false, Reason: err.Error()}
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return AppendAssistantMessageResult{OK: false, Reason: err.Error()}
	}

	// 更新 sessionFile 引用（如果不同）
	if entry.SessionFile == "" || entry.SessionFile != sessionFile {
		_ = store.Update(func(s map[string]*FullSessionEntry) error {
			if e, ok := s[sessionKey]; ok {
				e.SessionFile = sessionFile
			}
			return nil
		})
	}

	return AppendAssistantMessageResult{OK: true, SessionFile: sessionFile}
}

// AppendTranscriptParams 追加转录的参数。
type AppendTranscriptParams struct {
	AgentID    string
	SessionKey string
	Text       string
	MediaURLs  []string
	StorePath  string
}
