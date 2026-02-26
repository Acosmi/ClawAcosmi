package hooks

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================================
// Bundled hook handlers — Go 静态注册
// 对应 TS: bundled/boot-md/handler.ts, bundled/command-logger/handler.ts,
//         bundled/session-memory/handler.ts, bundled/soul-evil/handler.ts
//
// 注意：TS 版通过 import() 动态加载 handler.ts。Go 版将 handler 编译为静态函数，
// 在 RegisterBundledHooks() 中显式注册。
// ============================================================================

// RegisterBundledHooks 注册所有内置钩子处理函数
// 在 loader.go 的 LoadInternalHooks 中调用。
func RegisterBundledHooks() int {
	count := 0

	// boot-md: gateway:startup 时运行 boot checklist
	RegisterInternalHook("gateway:startup", bootMdHandler)
	count++

	// command-logger: 记录所有 command 事件
	RegisterInternalHook("command", commandLoggerHandler)
	count++

	// session-memory: /new 命令时保存会话上下文到 memory 文件
	RegisterInternalHook("command:new", sessionMemoryHandler)
	count++

	// soul-evil: agent:bootstrap 时注入 soul-evil override
	RegisterInternalHook("agent:bootstrap", soulEvilHandler)
	count++

	slog.Debug("Registered bundled hooks", "count", count)
	return count
}

// bootMdHandler boot-md 钩子
// 对应 TS: bundled/boot-md/handler.ts
func bootMdHandler(event *InternalHookEvent) error {
	if event.Type != HookEventGateway || event.Action != "startup" {
		return nil
	}
	// Boot checklist 逻辑由上层编排调用，此处仅记录事件
	slog.Debug("[boot-md] Gateway startup hook triggered")
	return nil
}

// commandLoggerHandler command-logger 钩子
// 对应 TS: bundled/command-logger/handler.ts
// W4 修复：从仅 slog 骨架升级为 JSONL 文件写入
func commandLoggerHandler(event *InternalHookEvent) error {
	if event.Type != HookEventCommand {
		return nil
	}
	slog.Info("[command-logger] Command event",
		"action", event.Action,
		"sessionKey", event.SessionKey,
	)

	// 写入 JSONL 日志文件
	ctx := event.Context
	if ctx == nil {
		return nil
	}
	workspaceDir, _ := ctx["workspaceDir"].(string)
	if workspaceDir == "" {
		return nil // 无 workspaceDir 则跳过文件写入，仅保留 slog
	}

	logsDir := filepath.Join(workspaceDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		slog.Warn("[command-logger] failed to create logs dir", "dir", logsDir, "err", err)
		return nil
	}

	ts := event.Timestamp
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}

	entry := map[string]interface{}{
		"type":       "command",
		"action":     event.Action,
		"sessionKey": event.SessionKey,
		"timestamp":  ts,
	}
	// 附加 context 中的命令详情
	if source, ok := ctx["commandSource"].(string); ok && source != "" {
		entry["source"] = source
	}
	if rawArgs, ok := ctx["commandArgs"].(string); ok && rawArgs != "" {
		entry["args"] = rawArgs
	}

	data, err := json.Marshal(entry)
	if err != nil {
		slog.Warn("[command-logger] failed to marshal entry", "err", err)
		return nil
	}

	logPath := filepath.Join(logsDir, "commands.jsonl")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Warn("[command-logger] failed to open log file", "path", logPath, "err", err)
		return nil
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		slog.Warn("[command-logger] failed to write log entry", "err", err)
	}
	return nil
}

// sessionMemoryHandler session-memory 钩子
// 对应 TS: bundled/session-memory/handler.ts
func sessionMemoryHandler(event *InternalHookEvent) error {
	if event.Type != HookEventCommand || event.Action != "new" {
		return nil
	}

	slog.Debug("[session-memory] /new command hook triggered",
		"sessionKey", event.SessionKey,
	)

	ctx := event.Context
	if ctx == nil {
		ctx = map[string]interface{}{}
	}

	// 1. 获取 stateDir / workspaceDir
	stateDir, _ := ctx["stateDir"].(string)
	workspaceDir, _ := ctx["workspaceDir"].(string)
	if workspaceDir == "" && stateDir != "" {
		workspaceDir = filepath.Join(stateDir, "workspace")
	}
	if workspaceDir == "" {
		slog.Warn("[session-memory] no workspaceDir in context, skipping")
		return nil
	}

	memoryDir := filepath.Join(workspaceDir, "memory")
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		return fmt.Errorf("session-memory: mkdir memory: %w", err)
	}

	// 2. 确定 session 文件路径
	var sessionFile string
	if se, ok := ctx["previousSessionEntry"].(map[string]interface{}); ok {
		sessionFile, _ = se["sessionFile"].(string)
	}
	if sessionFile == "" {
		if se, ok := ctx["sessionEntry"].(map[string]interface{}); ok {
			sessionFile, _ = se["sessionFile"].(string)
		}
	}

	// 确定消息数量上限（hookConfig 可通过 context 传入）
	messageCount := 15
	if hc, ok := ctx["hookConfig"].(map[string]interface{}); ok {
		if mc, ok := hc["messages"].(float64); ok && mc > 0 {
			messageCount = int(mc)
		}
	}

	// 3. 读取 JSONL transcript，提取最近 messageCount 条消息
	var sessionContent string
	if sessionFile != "" {
		content, err := readRecentSessionContent(sessionFile, messageCount)
		if err != nil {
			slog.Warn("[session-memory] failed to read session file",
				"file", sessionFile, "err", err)
		} else {
			sessionContent = content
		}
	}

	// 4. 时间戳
	now := time.UnixMilli(event.Timestamp)
	if event.Timestamp == 0 {
		now = time.Now()
	}
	dateStr := now.UTC().Format("2006-01-02")
	timeStr := now.UTC().Format("15:04:05")

	// 5. 生成 slug（无 LLM 客户端时退化为时间戳 slug）
	slug := now.UTC().Format("1504") // HHMM fallback
	// LLM 调用留给上层注入 llmClient，此处通过 context 可选传入
	if llmCli, ok := ctx["llmClient"].(LLMClient); ok && sessionContent != "" {
		messages := strings.Split(sessionContent, "\n")
		generated, err := GenerateSessionSlug(context.Background(), messages, llmCli)
		if err != nil {
			slog.Debug("[session-memory] slug generation failed, using fallback", "err", err)
		} else if generated != "" {
			slug = generated
		}
	}

	// 6. 确定 sessionId
	sessionId := "unknown"
	if se, ok := ctx["previousSessionEntry"].(map[string]interface{}); ok {
		if sid, ok := se["sessionId"].(string); ok && sid != "" {
			sessionId = sid
		}
	}
	if sessionId == "unknown" {
		if se, ok := ctx["sessionEntry"].(map[string]interface{}); ok {
			if sid, ok := se["sessionId"].(string); ok && sid != "" {
				sessionId = sid
			}
		}
	}

	source, _ := ctx["commandSource"].(string)
	if source == "" {
		source = "unknown"
	}

	// 7. 构造 Markdown 内容
	parts := []string{
		fmt.Sprintf("# Session: %s %s UTC", dateStr, timeStr),
		"",
		fmt.Sprintf("- **Session Key**: %s", event.SessionKey),
		fmt.Sprintf("- **Session ID**: %s", sessionId),
		fmt.Sprintf("- **Source**: %s", source),
		"",
	}
	if sessionContent != "" {
		parts = append(parts, "## Conversation Summary", "", sessionContent, "")
	}
	entry := strings.Join(parts, "\n")

	// 8. 写入 memory 文件
	filename := fmt.Sprintf("%s-%s.md", dateStr, slug)
	memoryFilePath := filepath.Join(memoryDir, filename)
	if err := os.WriteFile(memoryFilePath, []byte(entry), 0o644); err != nil {
		return fmt.Errorf("session-memory: write memory file: %w", err)
	}

	slog.Info("[session-memory] session context saved", "file", memoryFilePath)
	return nil
}

// sessionTranscriptEntry JSONL 行结构（与 TS handler 保持一致）
type sessionTranscriptEntry struct {
	Type    string `json:"type"`
	Message *struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// readRecentSessionContent 读取 JSONL session 文件，提取最近 messageCount 条 user/assistant 消息。
// 对应 TS: bundled/session-memory/handler.ts getRecentSessionContent
func readRecentSessionContent(sessionFilePath string, messageCount int) (string, error) {
	f, err := os.Open(sessionFilePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var allMessages []string
	scanner := bufio.NewScanner(f)
	// 支持较长行（最大 1 MiB）
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry sessionTranscriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Type != "message" || entry.Message == nil {
			continue
		}
		role := entry.Message.Role
		if role != "user" && role != "assistant" {
			continue
		}
		// content 可能是字符串或数组
		text := extractTextContent(entry.Message.Content)
		if text == "" || strings.HasPrefix(text, "/") {
			continue
		}
		allMessages = append(allMessages, role+": "+text)
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	// 取最后 messageCount 条
	if len(allMessages) > messageCount {
		allMessages = allMessages[len(allMessages)-messageCount:]
	}
	return strings.Join(allMessages, "\n"), nil
}

// extractTextContent 从 JSON content（字符串或 content block 数组）中提取纯文本。
func extractTextContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// 尝试作为字符串解析
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// 尝试作为数组（content blocks）解析，取第一个 type=="text" 的块
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				return b.Text
			}
		}
	}
	return ""
}

// soulEvilHandler soul-evil 钩子
// 对应 TS: bundled/soul-evil/handler.ts
// 接通 soul_evil.go ApplySoulEvilOverride — W4-14 修复
func soulEvilHandler(event *InternalHookEvent) error {
	if !IsAgentBootstrapEvent(event) {
		return nil
	}

	ctx := event.Context
	if ctx == nil {
		return nil
	}

	// 1. 获取 workspaceDir
	workspaceDir, _ := ctx["workspaceDir"].(string)
	if workspaceDir == "" {
		slog.Debug("[soul-evil] no workspaceDir in context, skipping")
		return nil
	}

	// 2. 解析 soul-evil 配置（从 hookConfig 或 agentConfig 中获取）
	var soulEvilCfg *SoulEvilConfig
	log := &SoulEvilLog{
		Debug: func(msg string) { slog.Debug("[soul-evil] " + msg) },
		Warn:  func(msg string) { slog.Warn("[soul-evil] " + msg) },
	}
	if hc, ok := ctx["hookConfig"].(map[string]interface{}); ok {
		if seEntry, ok := hc["soul-evil"].(map[string]interface{}); ok {
			soulEvilCfg = ResolveSoulEvilConfigFromHook(seEntry, log)
		}
	}
	// 无配置则无需处理
	if soulEvilCfg == nil {
		slog.Debug("[soul-evil] no soul-evil config found, skipping")
		return nil
	}

	// 3. 解析 workspace bootstrap files
	var files []WorkspaceBootstrapFile
	if rawFiles, ok := ctx["workspaceBootstrapFiles"].([]interface{}); ok {
		for _, rf := range rawFiles {
			if fm, ok := rf.(map[string]interface{}); ok {
				name, _ := fm["name"].(string)
				content, _ := fm["content"].(string)
				missing, _ := fm["missing"].(bool)
				files = append(files, WorkspaceBootstrapFile{
					Name:    name,
					Content: content,
					Missing: missing,
				})
			}
		}
	}
	if len(files) == 0 {
		slog.Debug("[soul-evil] no bootstrap files in context, skipping")
		return nil
	}

	// 4. 获取用户时区
	userTimezone, _ := ctx["userTimezone"].(string)

	// 5. 调用 ApplySoulEvilOverride
	result := ApplySoulEvilOverride(ApplySoulEvilParams{
		Files:        files,
		WorkspaceDir: workspaceDir,
		Config:       soulEvilCfg,
		UserTimezone: userTimezone,
		Log:          log,
	})

	// 6. 回写结果到 context
	resultFiles := make([]interface{}, len(result))
	for i, f := range result {
		resultFiles[i] = map[string]interface{}{
			"name":    f.Name,
			"content": f.Content,
			"missing": f.Missing,
		}
	}
	ctx["workspaceBootstrapFiles"] = resultFiles

	slog.Debug("[soul-evil] handler completed",
		"sessionKey", event.SessionKey,
		"filesCount", len(result),
	)
	return nil
}
