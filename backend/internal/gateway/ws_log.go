package gateway

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// ---------- WS 日志子系统 ----------
// 对齐 TS ws-log.ts + ws-logging.ts
// 3 种模式: auto (仅错误/慢请求), compact (合并 req/res), full (全帧)

// WsLogStyle WS 日志风格。
type WsLogStyle string

const (
	// WsLogStyleAuto 默认模式: 仅记录错误响应和慢请求 (>50ms)。
	WsLogStyleAuto WsLogStyle = "auto"
	// WsLogStyleCompact 紧凑模式: 合并 req/res 对，省略重复 connId。
	WsLogStyleCompact WsLogStyle = "compact"
	// WsLogStyleFull 完整模式: 记录每个帧的完整信息。
	WsLogStyleFull WsLogStyle = "full"
)

// DefaultWsSlowMs 慢请求阈值（毫秒），对齐 TS DEFAULT_WS_SLOW_MS = 50。
const DefaultWsSlowMs = 50

// LogValueLimit 日志值最大长度，对齐 TS LOG_VALUE_LIMIT = 240。
const LogValueLimit = 240

// wsLogState WS 日志全局状态。
type wsLogState struct {
	mu    sync.Mutex
	style WsLogStyle

	// auto 模式: inflight req 时间戳
	inflightOptimized map[string]int64

	// compact 模式: inflight req 时间戳 + 元数据
	inflightCompact map[string]wsInflightEntry
	lastCompactConn string
}

type wsInflightEntry struct {
	ts     int64
	method string
}

var globalWsLog = &wsLogState{
	style:             WsLogStyleAuto,
	inflightOptimized: make(map[string]int64),
	inflightCompact:   make(map[string]wsInflightEntry),
}

// SetWsLogStyle 设置 WS 日志风格。
func SetWsLogStyle(style WsLogStyle) {
	globalWsLog.mu.Lock()
	defer globalWsLog.mu.Unlock()
	globalWsLog.style = style
}

// GetWsLogStyle 获取当前 WS 日志风格。
func GetWsLogStyle() WsLogStyle {
	globalWsLog.mu.Lock()
	defer globalWsLog.mu.Unlock()
	return globalWsLog.style
}

// LogWs 记录 WS 帧日志。
// direction: "in" (客户端→服务端) | "out" (服务端→客户端)
// kind: "req" | "res" | "event" | "open" | "close" | "parse-error"
// meta: 可选的结构化元数据
func LogWs(direction, kind string, meta map[string]interface{}) {
	globalWsLog.mu.Lock()
	style := globalWsLog.style
	globalWsLog.mu.Unlock()

	switch style {
	case WsLogStyleFull:
		logWsFull(direction, kind, meta)
	case WsLogStyleCompact:
		logWsCompact(direction, kind, meta)
	default:
		logWsOptimized(direction, kind, meta)
	}
}

// ---------- auto (optimized) 模式 ----------
// 对齐 TS logWsOptimized: 仅记录失败响应和慢请求

func logWsOptimized(direction, kind string, meta map[string]interface{}) {
	connID := metaString(meta, "connId")
	id := metaString(meta, "id")
	method := metaString(meta, "method")
	ok := metaBool(meta, "ok")
	inflightKey := inflightKeyFrom(connID, id)

	// 记录入站请求时间
	if direction == "in" && kind == "req" && inflightKey != "" {
		globalWsLog.mu.Lock()
		globalWsLog.inflightOptimized[inflightKey] = time.Now().UnixMilli()
		if len(globalWsLog.inflightOptimized) > 2000 {
			globalWsLog.inflightOptimized = make(map[string]int64)
		}
		globalWsLog.mu.Unlock()
		return
	}

	// 记录解析错误
	if kind == "parse-error" {
		errorMsg := metaString(meta, "error")
		slog.Warn("ws parse-error",
			"conn", ShortID(connID),
			"error", errorMsg,
		)
		return
	}

	// 仅处理出站响应
	if direction != "out" || kind != "res" {
		return
	}

	// 计算耗时
	var durationMs int64 = -1
	if inflightKey != "" {
		globalWsLog.mu.Lock()
		if startedAt, exists := globalWsLog.inflightOptimized[inflightKey]; exists {
			durationMs = time.Now().UnixMilli() - startedAt
			delete(globalWsLog.inflightOptimized, inflightKey)
		}
		globalWsLog.mu.Unlock()
	}

	// 仅记录失败或慢请求
	shouldLog := (ok != nil && !*ok) || (durationMs >= DefaultWsSlowMs)
	if !shouldLog {
		return
	}

	attrs := []any{
		"kind", "res",
		"method", method,
	}
	if ok != nil {
		attrs = append(attrs, "ok", *ok)
	}
	if durationMs >= 0 {
		attrs = append(attrs, "durationMs", durationMs)
	}
	attrs = append(attrs, "conn", ShortID(connID))
	if id != "" {
		attrs = append(attrs, "id", ShortID(id))
	}
	appendExtraMeta(&attrs, meta, "connId", "id", "method", "ok")

	slog.Info("ws", attrs...)
}

// ---------- compact 模式 ----------
// 对齐 TS logWsCompact: 合并 req/res 对

func logWsCompact(direction, kind string, meta map[string]interface{}) {
	now := time.Now().UnixMilli()
	connID := metaString(meta, "connId")
	id := metaString(meta, "id")
	method := metaString(meta, "method")
	ok := metaBool(meta, "ok")
	event := metaString(meta, "event")
	inflightKey := inflightKeyFrom(connID, id)

	// 收集入站请求，不立即输出
	if kind == "req" && direction == "in" && inflightKey != "" {
		globalWsLog.mu.Lock()
		globalWsLog.inflightCompact[inflightKey] = wsInflightEntry{ts: now, method: method}
		globalWsLog.mu.Unlock()
		return
	}

	// 计算耗时
	var durationMs int64 = -1
	if kind == "res" && direction == "out" && inflightKey != "" {
		globalWsLog.mu.Lock()
		if entry, exists := globalWsLog.inflightCompact[inflightKey]; exists {
			durationMs = now - entry.ts
			if method == "" {
				method = entry.method
			}
			delete(globalWsLog.inflightCompact, inflightKey)
		}
		globalWsLog.mu.Unlock()
	}

	// 构建日志
	dirArrow := directionArrow(direction, kind)
	attrs := []any{
		"dir", dirArrow,
		"kind", kind,
	}

	// headline
	if (kind == "req" || kind == "res") && method != "" {
		attrs = append(attrs, "method", method)
	} else if kind == "event" && event != "" {
		attrs = append(attrs, "event", event)
	}

	if kind == "res" && ok != nil {
		attrs = append(attrs, "ok", *ok)
	}
	if durationMs >= 0 {
		attrs = append(attrs, "durationMs", durationMs)
	}

	// 仅变化时输出 connId
	globalWsLog.mu.Lock()
	showConn := connID != "" && connID != globalWsLog.lastCompactConn
	if connID != "" {
		globalWsLog.lastCompactConn = connID
	}
	globalWsLog.mu.Unlock()

	if showConn {
		attrs = append(attrs, "conn", ShortID(connID))
	}
	if id != "" {
		attrs = append(attrs, "id", ShortID(id))
	}
	appendExtraMeta(&attrs, meta, "connId", "id", "method", "ok", "event")

	slog.Info("ws", attrs...)
}

// ---------- full 模式 ----------
// 对齐 TS logWs (verbose + full): 记录每个帧的完整信息

func logWsFull(direction, kind string, meta map[string]interface{}) {
	connID := metaString(meta, "connId")
	id := metaString(meta, "id")
	method := metaString(meta, "method")
	ok := metaBool(meta, "ok")
	event := metaString(meta, "event")
	inflightKey := inflightKeyFrom(connID, id)

	// 记录入站请求时间
	if direction == "in" && kind == "req" && inflightKey != "" {
		globalWsLog.mu.Lock()
		globalWsLog.inflightOptimized[inflightKey] = time.Now().UnixMilli()
		globalWsLog.mu.Unlock()
	}

	// 计算耗时
	var durationMs int64 = -1
	if direction == "out" && kind == "res" && inflightKey != "" {
		globalWsLog.mu.Lock()
		if startedAt, exists := globalWsLog.inflightOptimized[inflightKey]; exists {
			durationMs = time.Now().UnixMilli() - startedAt
			delete(globalWsLog.inflightOptimized, inflightKey)
		}
		globalWsLog.mu.Unlock()
	}

	dirArrow := directionArrow(direction, kind)
	attrs := []any{
		"dir", dirArrow,
		"kind", kind,
	}

	if (kind == "req" || kind == "res") && method != "" {
		attrs = append(attrs, "method", method)
	} else if kind == "event" && event != "" {
		attrs = append(attrs, "event", event)
	}

	if kind == "res" && ok != nil {
		attrs = append(attrs, "ok", *ok)
	}
	if durationMs >= 0 {
		attrs = append(attrs, "durationMs", durationMs)
	}
	if connID != "" {
		attrs = append(attrs, "conn", ShortID(connID))
	}
	if id != "" {
		attrs = append(attrs, "id", ShortID(id))
	}
	appendExtraMeta(&attrs, meta, "connId", "id", "method", "ok", "event")

	slog.Info("ws", attrs...)
}

// ---------- 辅助函数 ----------

// ShortID 缩短 UUID/长字符串用于日志显示。
// 对齐 TS shortId(): UUID → "xxxxxxxx…xxxx", 长字符串 → "xxxxxxxxxxxx…xxxx"。
func ShortID(value string) string {
	s := strings.TrimSpace(value)
	if s == "" {
		return "?"
	}
	// UUID 格式检查: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	if len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-' {
		return s[:8] + "…" + s[len(s)-4:]
	}
	if utf8.RuneCountInString(s) <= 24 {
		return s
	}
	runes := []rune(s)
	return string(runes[:12]) + "…" + string(runes[len(runes)-4:])
}

// FormatForLog 格式化值用于日志输出，截断超长内容。
// 对齐 TS formatForLog()。
func FormatForLog(value interface{}) string {
	if value == nil {
		return ""
	}
	var str string
	switch v := value.(type) {
	case string:
		str = v
	case error:
		str = v.Error()
	case fmt.Stringer:
		str = v.String()
	default:
		data, err := json.Marshal(v)
		if err != nil {
			str = fmt.Sprintf("%v", v)
		} else {
			str = string(data)
		}
	}
	if len(str) == 0 {
		return ""
	}
	if len(str) > LogValueLimit {
		return str[:LogValueLimit] + "..."
	}
	return str
}

func inflightKeyFrom(connID, id string) string {
	if connID == "" || id == "" {
		return ""
	}
	return connID + ":" + id
}

func directionArrow(direction, kind string) string {
	if kind == "req" || kind == "res" {
		return "⇄"
	}
	if direction == "in" {
		return "←"
	}
	return "→"
}

func metaString(meta map[string]interface{}, key string) string {
	if meta == nil {
		return ""
	}
	v, ok := meta[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func metaBool(meta map[string]interface{}, key string) *bool {
	if meta == nil {
		return nil
	}
	v, ok := meta[key]
	if !ok {
		return nil
	}
	b, ok := v.(bool)
	if !ok {
		return nil
	}
	return &b
}

func appendExtraMeta(attrs *[]any, meta map[string]interface{}, skip ...string) {
	if meta == nil {
		return
	}
	skipSet := make(map[string]struct{}, len(skip))
	for _, k := range skip {
		skipSet[k] = struct{}{}
	}
	for k, v := range meta {
		if _, skipped := skipSet[k]; skipped {
			continue
		}
		if v == nil {
			continue
		}
		*attrs = append(*attrs, k, FormatForLog(v))
	}
}

// ---------- Agent 事件摘要 ----------
// 对齐 TS ws-log.ts L103-191: summarizeAgentEventForWsLog()

// CompactPreview 折叠多行文本为单行预览，超长截断。
// 对齐 TS ws-log.ts L95-101: compactPreview()
func CompactPreview(input string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 160
	}
	// 折叠所有连续空白为单个空格
	oneLine := strings.Join(strings.Fields(input), " ")
	if utf8.RuneCountInString(oneLine) <= maxLen {
		return oneLine
	}
	runes := []rune(oneLine)
	return string(runes[:maxLen-1]) + "…"
}

// SummarizeAgentEventForWsLog 从 agent 事件 payload 中提取结构化摘要。
// 对齐 TS ws-log.ts L103-191: summarizeAgentEventForWsLog()
func SummarizeAgentEventForWsLog(payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		return map[string]interface{}{}
	}

	runID := metaString(payload, "runId")
	stream := metaString(payload, "stream")
	sessionKey := metaString(payload, "sessionKey")
	extra := make(map[string]interface{})

	// 基础字段
	if runID != "" {
		extra["run"] = ShortID(runID)
	}
	if sessionKey != "" {
		parsed := parseAgentSessionKeySimple(sessionKey)
		if parsed != nil {
			extra["agent"] = parsed.agentId
			extra["session"] = parsed.rest
		} else {
			extra["session"] = sessionKey
		}
	}
	if stream != "" {
		extra["stream"] = stream
	}
	if seq, ok := payload["seq"]; ok {
		if seqNum, isNum := seq.(float64); isNum {
			extra["aseq"] = int(seqNum)
		} else if seqInt, isInt := seq.(int); isInt {
			extra["aseq"] = seqInt
		} else if seqInt64, isInt64 := seq.(int64); isInt64 {
			extra["aseq"] = seqInt64
		}
	}

	data, _ := payload["data"].(map[string]interface{})
	if data == nil {
		return extra
	}

	// stream == "assistant"
	if stream == "assistant" {
		if text, ok := data["text"].(string); ok && strings.TrimSpace(text) != "" {
			extra["text"] = CompactPreview(text, 160)
		}
		if mediaUrls, ok := data["mediaUrls"].([]interface{}); ok && len(mediaUrls) > 0 {
			extra["media"] = len(mediaUrls)
		}
		return extra
	}

	// stream == "tool"
	if stream == "tool" {
		phase, _ := data["phase"].(string)
		name, _ := data["name"].(string)
		if phase != "" || name != "" {
			p := phase
			if p == "" {
				p = "?"
			}
			n := name
			if n == "" {
				n = "?"
			}
			extra["tool"] = p + ":" + n
		}
		if toolCallID, ok := data["toolCallId"].(string); ok && toolCallID != "" {
			extra["call"] = ShortID(toolCallID)
		}
		if meta, ok := data["meta"].(string); ok && strings.TrimSpace(meta) != "" {
			extra["meta"] = meta
		}
		if isError, ok := data["isError"].(bool); ok {
			extra["err"] = isError
		}
		return extra
	}

	// stream == "lifecycle"
	if stream == "lifecycle" {
		if phase, ok := data["phase"].(string); ok && phase != "" {
			extra["phase"] = phase
		}
		if aborted, ok := data["aborted"].(bool); ok {
			extra["aborted"] = aborted
		}
		if errStr, ok := data["error"].(string); ok && strings.TrimSpace(errStr) != "" {
			extra["error"] = CompactPreview(errStr, 120)
		}
		return extra
	}

	// 其他 stream
	if reason, ok := data["reason"].(string); ok && strings.TrimSpace(reason) != "" {
		extra["reason"] = reason
	}
	return extra
}

// ---------- 敏感信息脱敏 ----------
// 对齐 TS logging/redact.ts

const (
	redactMinLength = 18
	redactKeepStart = 6
	redactKeepEnd   = 4
)

// defaultRedactPatterns 默认脱敏正则模式。
// 对齐 TS redact.ts: DEFAULT_REDACT_PATTERNS (15 个模式)
var defaultRedactPatterns = []*regexp.Regexp{
	// 1. ENV-style assignments
	regexp.MustCompile(`(?i)\b[A-Z0-9_]*(?:KEY|TOKEN|SECRET|PASSWORD|PASSWD)\b\s*[=:]\s*["']?([^\s"'\\]+)["']?`),
	// 2. JSON fields
	regexp.MustCompile(`(?i)"(?:apiKey|token|secret|password|passwd|accessToken|refreshToken)"\s*:\s*"([^"]+)"`),
	// 3. CLI flags
	regexp.MustCompile(`(?i)--(?:api[-_]?key|token|secret|password|passwd)\s+["']?([^\s"']+)["']?`),
	// 4. Authorization: Bearer
	regexp.MustCompile(`(?i)Authorization\s*[:=]\s*Bearer\s+([A-Za-z0-9._\-+=]+)`),
	// 5. Bearer token standalone
	regexp.MustCompile(`(?i)\bBearer\s+([A-Za-z0-9._\-+=]{18,})\b`),
	// 6. PEM blocks
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]+?-----END [A-Z ]*PRIVATE KEY-----`),
	// 7-15. Common token prefixes
	regexp.MustCompile(`\b(sk-[A-Za-z0-9_-]{8,})\b`),
	regexp.MustCompile(`\b(ghp_[A-Za-z0-9]{20,})\b`),
	regexp.MustCompile(`\b(github_pat_[A-Za-z0-9_]{20,})\b`),
	regexp.MustCompile(`\b(xox[baprs]-[A-Za-z0-9-]{10,})\b`),
	regexp.MustCompile(`\b(xapp-[A-Za-z0-9-]{10,})\b`),
	regexp.MustCompile(`\b(gsk_[A-Za-z0-9_-]{10,})\b`),
	regexp.MustCompile(`\b(AIza[0-9A-Za-z\-_]{20,})\b`),
	regexp.MustCompile(`\b(pplx-[A-Za-z0-9_-]{10,})\b`),
	regexp.MustCompile(`\b(npm_[A-Za-z0-9]{10,})\b`),
}

// maskToken 遮蔽 token，保留前后部分。
// 对齐 TS redact.ts: maskToken()
func maskToken(token string) string {
	if len(token) < redactMinLength {
		return "***"
	}
	runes := []rune(token)
	if len(runes) < redactMinLength {
		return "***"
	}
	start := string(runes[:redactKeepStart])
	end := string(runes[len(runes)-redactKeepEnd:])
	return start + "…" + end
}

// redactPemBlock 遮蔽 PEM 块。
// 对齐 TS redact.ts: redactPemBlock()
func redactPemBlock(block string) string {
	lines := strings.Split(block, "\n")
	var nonEmpty []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			nonEmpty = append(nonEmpty, trimmed)
		}
	}
	if len(nonEmpty) < 2 {
		return "***"
	}
	return nonEmpty[0] + "\n…redacted…\n" + nonEmpty[len(nonEmpty)-1]
}

// RedactSensitiveText 对文本中的敏感信息进行脱敏。
// 对齐 TS redact.ts: redactSensitiveText()
func RedactSensitiveText(text string) string {
	if text == "" {
		return text
	}
	result := text
	for _, pattern := range defaultRedactPatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			// PEM block 特殊处理
			if strings.Contains(match, "PRIVATE KEY-----") {
				return redactPemBlock(match)
			}

			// 尝试提取捕获组
			subs := pattern.FindStringSubmatch(match)
			if len(subs) > 1 {
				// 找到最后一个非空捕获组
				token := ""
				for i := len(subs) - 1; i >= 1; i-- {
					if subs[i] != "" {
						token = subs[i]
						break
					}
				}
				if token != "" {
					masked := maskToken(token)
					if token == match {
						return masked
					}
					return strings.Replace(match, token, masked, 1)
				}
			}

			// 无捕获组，遮蔽整个匹配
			return maskToken(match)
		})
	}
	return result
}
