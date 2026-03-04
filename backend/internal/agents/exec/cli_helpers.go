package exec

// ============================================================================
// CLI Runner 辅助函数
// 对应 TS: agents/cli-runner/helpers.ts (输出解析 + 参数构建 + 队列)
// ============================================================================

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// DefaultCliQueueTimeout 队列锁获取的默认超时（5 分钟）。
const DefaultCliQueueTimeout = 5 * time.Minute

// CliUsage CLI 运行使用量。
type CliUsage struct {
	Input      *int `json:"input,omitempty"`
	Output     *int `json:"output,omitempty"`
	CacheRead  *int `json:"cacheRead,omitempty"`
	CacheWrite *int `json:"cacheWrite,omitempty"`
	Total      *int `json:"total,omitempty"`
}

// CliOutput CLI 运行的解析结果。
type CliOutput struct {
	Text      string    `json:"text"`
	SessionID string    `json:"sessionId,omitempty"`
	Usage     *CliUsage `json:"usage,omitempty"`
}

// --- CLI 运行队列 (隐藏依赖 #2: 全局状态/单例) ---

var (
	cliQueueMu sync.Mutex
	cliQueues  = map[string]*sync.Mutex{}
)

// EnqueueCliRun 串行化同一 backend key 的 CLI 调用。
// 添加 context.WithTimeout 保护，防止前一个任务挂起导致后续任务永久等待。
func EnqueueCliRun(key string, task func() (*CliOutput, error)) (*CliOutput, error) {
	return EnqueueCliRunWithContext(context.Background(), key, task)
}

// EnqueueCliRunWithContext 带上下文的串行化 CLI 调用。
// 如果上下文无截止时间，自动添加 DefaultCliQueueTimeout。
func EnqueueCliRunWithContext(ctx context.Context, key string, task func() (*CliOutput, error)) (*CliOutput, error) {
	cliQueueMu.Lock()
	mu, ok := cliQueues[key]
	if !ok {
		mu = &sync.Mutex{}
		cliQueues[key] = mu
	}
	cliQueueMu.Unlock()

	// 确保有超时保护
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultCliQueueTimeout)
		defer cancel()
	}

	// 带超时的锁获取（轮询方式，因 sync.Mutex 不支持 TryLock with context）
	locked := make(chan struct{}, 1)
	go func() {
		mu.Lock()
		locked <- struct{}{}
	}()

	select {
	case <-locked:
		defer mu.Unlock()
		return task()
	case <-ctx.Done():
		// 超时：启动 goroutine 在锁可用时释放，避免泄漏
		go func() {
			<-locked
			mu.Unlock()
		}()
		return nil, fmt.Errorf("cli queue timeout for key %q: %w", key, ctx.Err())
	}
}

// --- 模型别名 ---

// NormalizeCliModel 应用 backend 的 modelAliases。
func NormalizeCliModel(modelID string, backend *types.CliBackendConfig) string {
	trimmed := strings.TrimSpace(modelID)
	if backend.ModelAliases != nil {
		if alias, ok := backend.ModelAliases[trimmed]; ok {
			return alias
		}
	}
	return trimmed
}

// --- 输出解析 ---

// ParseCliJson 从 JSON 输出解析文本和 sessionId。
func ParseCliJson(raw string, backend *types.CliBackendConfig) *CliOutput {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil
	}
	text := collectText(parsed)
	sessionID := pickSessionID(parsed, backend)
	usage := toUsage(parsed)
	return &CliOutput{Text: text, SessionID: sessionID, Usage: usage}
}

// ParseCliJsonl 从 JSONL 输出解析文本。
func ParseCliJsonl(raw string, backend *types.CliBackendConfig) *CliOutput {
	lines := strings.Split(raw, "\n")
	var texts []string
	var sessionID string
	var usage *CliUsage
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			continue
		}
		t := collectText(parsed)
		if t != "" {
			texts = append(texts, t)
		}
		if sessionID == "" {
			sessionID = pickSessionID(parsed, backend)
		}
		if usage == nil {
			usage = toUsage(parsed)
		}
	}
	if len(texts) == 0 {
		return nil
	}
	return &CliOutput{
		Text:      strings.Join(texts, "\n"),
		SessionID: sessionID,
		Usage:     usage,
	}
}

func collectText(parsed map[string]interface{}) string {
	// 优先取 result/text 等常见字段
	for _, key := range []string{"result", "text", "content", "response", "output", "message"} {
		if v, ok := parsed[key]; ok {
			switch val := v.(type) {
			case string:
				return val
			case map[string]interface{}:
				if inner, ok := val["text"]; ok {
					if s, ok := inner.(string); ok {
						return s
					}
				}
			}
		}
	}
	return ""
}

func pickSessionID(parsed map[string]interface{}, backend *types.CliBackendConfig) string {
	fields := backend.SessionIDFields
	if len(fields) == 0 {
		fields = []string{"session_id", "sessionId"}
	}
	for _, field := range fields {
		if v, ok := parsed[field]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func toUsage(parsed map[string]interface{}) *CliUsage {
	raw, ok := parsed["usage"]
	if !ok {
		return nil
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	pick := func(key string) *int {
		if v, ok := m[key]; ok {
			if f, ok := v.(float64); ok && f > 0 {
				i := int(f)
				return &i
			}
		}
		return nil
	}
	u := &CliUsage{
		Input:      pick("input"),
		Output:     pick("output"),
		CacheRead:  pick("cacheRead"),
		CacheWrite: pick("cacheWrite"),
		Total:      pick("total"),
	}
	if u.Input == nil && u.Output == nil && u.Total == nil {
		return nil
	}
	return u
}

// --- 参数构建 ---

// BuildCliArgs 构建 CLI 命令行参数。
func BuildCliArgs(backend *types.CliBackendConfig, baseArgs []string, modelID string, sessionID string, systemPrompt string, imagePaths []string, promptArg string, useResume bool) []string {
	args := append([]string(nil), baseArgs...)
	if backend.ModelArg != "" && modelID != "" {
		args = append(args, backend.ModelArg, modelID)
	}
	if sessionID != "" && !useResume {
		if backend.SessionArg != "" {
			args = append(args, backend.SessionArg, sessionID)
		} else if len(backend.SessionArgs) > 0 {
			args = append(args, backend.SessionArgs...)
		}
	}
	if systemPrompt != "" && backend.SystemPromptArg != "" {
		args = append(args, backend.SystemPromptArg, systemPrompt)
	}
	if len(imagePaths) > 0 && backend.ImageArg != "" {
		if backend.ImageMode == "repeat" {
			for _, p := range imagePaths {
				args = append(args, backend.ImageArg, p)
			}
		} else {
			args = append(args, backend.ImageArg, strings.Join(imagePaths, ","))
		}
	}
	if promptArg != "" {
		args = append(args, promptArg)
	}
	return args
}

// ResolvePromptInput 决定 prompt 是通过参数还是 stdin 传递。
func ResolvePromptInput(backend *types.CliBackendConfig, prompt string) (argsPrompt string, stdin string) {
	if backend.Input == "stdin" {
		return "", prompt
	}
	maxChars := 0
	if backend.MaxPromptArgChars != nil {
		maxChars = *backend.MaxPromptArgChars
	}
	if maxChars > 0 && len(prompt) > maxChars {
		return "", prompt
	}
	return prompt, ""
}

// ResolveSessionIDToSend 解析要发送给 CLI 的 sessionId。
func ResolveSessionIDToSend(backend *types.CliBackendConfig, cliSessionID string) (sessionID string, isNew bool) {
	if cliSessionID != "" {
		return cliSessionID, false
	}
	if backend.SessionMode == "always" {
		return fmt.Sprintf("oc-%d", os.Getpid()), true
	}
	return "", true
}

// ResolveSystemPromptUsage 决定是否发送系统提示词。
func ResolveSystemPromptUsage(backend *types.CliBackendConfig, isNewSession bool, systemPrompt string) string {
	if backend.SystemPromptArg == "" || systemPrompt == "" {
		return ""
	}
	when := backend.SystemPromptWhen
	if when == "never" {
		return ""
	}
	if when == "first" && !isNewSession {
		return ""
	}
	return systemPrompt
}
