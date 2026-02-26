package extensions

// compaction_safeguard.go — 紧凑保护扩展
// 对应 TS: agents/pi-extensions/compaction-safeguard.ts (347L)
//
// 提供 session compaction 保护：工具失败收集、文件操作追踪、
// 自适应分块摘要、split-turn 处理。

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ---------- 常量 ----------

const FallbackSummary = "Summary unavailable due to context limits. Older messages were truncated."
const TurnPrefixInstructions = "This summary covers the prefix of a split turn. Focus on the original request, early progress, and any details needed to understand the retained suffix."
const MaxToolFailures = 8
const MaxToolFailureChars = 240

// 自适应分块比率常量
const (
	BaseChunkRatio = 0.25
	MinChunkRatio  = 0.10
	SafetyMargin   = 0.85
)

// ---------- 类型 ----------

// ToolFailure 工具失败记录。
type ToolFailure struct {
	ToolCallID string `json:"toolCallId"`
	ToolName   string `json:"toolName"`
	Summary    string `json:"summary"`
	Meta       string `json:"meta,omitempty"`
}

// FileOperations 文件操作记录。
type FileOperations struct {
	Read    []string `json:"read"`
	Written []string `json:"written"`
	Edited  []string `json:"edited"`
}

// AgentMessage 简化的 agent 消息（用于 compaction 接口）。
type AgentMessage struct {
	Role       string          `json:"role"`
	ToolCallID string          `json:"toolCallId,omitempty"`
	ToolName   string          `json:"toolName,omitempty"`
	Content    json.RawMessage `json:"content,omitempty"`
	Details    json.RawMessage `json:"details,omitempty"`
	IsError    bool            `json:"isError,omitempty"`
	// CachedAt 记录消息被缓存的时间，用于 cache-ttl 剪枝模式。
	// 对应 TS AgentMessage 的 cachedAt 字段。
	CachedAt *time.Time `json:"cachedAt,omitempty"`
}

// CompactionPreparation 紧凑准备数据。
type CompactionPreparation struct {
	MessageToSummarize []AgentMessage `json:"messagesToSummarize"`
	TurnPrefixMessages []AgentMessage `json:"turnPrefixMessages"`
	FirstKeptEntryID   string         `json:"firstKeptEntryId"`
	TokensBefore       int            `json:"tokensBefore"`
	PreviousSummary    string         `json:"previousSummary,omitempty"`
	IsSplitTurn        bool           `json:"isSplitTurn"`
	FileOps            FileOperations `json:"fileOps"`
	Settings           struct {
		ReserveTokens int `json:"reserveTokens"`
	} `json:"settings"`
}

// CompactionResult 紧凑结果。
type CompactionResult struct {
	Summary          string   `json:"summary"`
	FirstKeptEntryID string   `json:"firstKeptEntryId"`
	TokensBefore     int      `json:"tokensBefore"`
	ReadFiles        []string `json:"readFiles,omitempty"`
	ModifiedFiles    []string `json:"modifiedFiles,omitempty"`
}

// ---------- 工具失败处理 ----------

var whitespaceRegex = regexp.MustCompile(`\s+`)

// NormalizeFailureText 规范化失败文本。
func NormalizeFailureText(text string) string {
	return strings.TrimSpace(whitespaceRegex.ReplaceAllString(text, " "))
}

// TruncateFailureText 截断失败文本。
func TruncateFailureText(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	if maxChars <= 3 {
		return "..."
	}
	return text[:maxChars-3] + "..."
}

// FormatToolFailureMeta 格式化工具失败元数据。
func FormatToolFailureMeta(details json.RawMessage) string {
	if len(details) == 0 {
		return ""
	}
	var record map[string]interface{}
	if err := json.Unmarshal(details, &record); err != nil {
		return ""
	}

	var parts []string
	if status, ok := record["status"].(string); ok && status != "" {
		parts = append(parts, fmt.Sprintf("status=%s", status))
	}
	if exitCode, ok := record["exitCode"].(float64); ok && math.IsInf(exitCode, 0) == false && !math.IsNaN(exitCode) {
		parts = append(parts, fmt.Sprintf("exitCode=%d", int(exitCode)))
	}
	return strings.Join(parts, " ")
}

// ExtractToolResultText 提取工具结果文本。
func ExtractToolResultText(content json.RawMessage) string {
	if len(content) == 0 {
		return ""
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(content, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, block := range blocks {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// CollectToolFailures 收集工具失败记录。
func CollectToolFailures(messages []AgentMessage) []ToolFailure {
	var failures []ToolFailure
	seen := make(map[string]bool)

	for _, msg := range messages {
		if msg.Role != "toolResult" || !msg.IsError {
			continue
		}
		if msg.ToolCallID == "" || seen[msg.ToolCallID] {
			continue
		}
		seen[msg.ToolCallID] = true

		toolName := msg.ToolName
		if strings.TrimSpace(toolName) == "" {
			toolName = "tool"
		}

		rawText := ExtractToolResultText(msg.Content)
		meta := FormatToolFailureMeta(msg.Details)
		normalized := NormalizeFailureText(rawText)
		if normalized == "" {
			if meta != "" {
				normalized = "failed"
			} else {
				normalized = "failed (no output)"
			}
		}
		summary := TruncateFailureText(normalized, MaxToolFailureChars)
		failures = append(failures, ToolFailure{
			ToolCallID: msg.ToolCallID,
			ToolName:   toolName,
			Summary:    summary,
			Meta:       meta,
		})
	}
	return failures
}

// FormatToolFailuresSection 格式化工具失败区段。
func FormatToolFailuresSection(failures []ToolFailure) string {
	if len(failures) == 0 {
		return ""
	}
	limit := MaxToolFailures
	if limit > len(failures) {
		limit = len(failures)
	}
	var lines []string
	for _, f := range failures[:limit] {
		meta := ""
		if f.Meta != "" {
			meta = fmt.Sprintf(" (%s)", f.Meta)
		}
		lines = append(lines, fmt.Sprintf("- %s%s: %s", f.ToolName, meta, f.Summary))
	}
	if len(failures) > MaxToolFailures {
		lines = append(lines, fmt.Sprintf("- ...and %d more", len(failures)-MaxToolFailures))
	}
	return fmt.Sprintf("\n\n## Tool Failures\n%s", strings.Join(lines, "\n"))
}

// ---------- 文件操作处理 ----------

// ComputeFileLists 计算文件列表（已读 / 已修改）。
func ComputeFileLists(fileOps FileOperations) (readFiles, modifiedFiles []string) {
	modified := make(map[string]bool)
	for _, f := range fileOps.Edited {
		modified[f] = true
	}
	for _, f := range fileOps.Written {
		modified[f] = true
	}

	for _, f := range fileOps.Read {
		if !modified[f] {
			readFiles = append(readFiles, f)
		}
	}
	for f := range modified {
		modifiedFiles = append(modifiedFiles, f)
	}

	sort.Strings(readFiles)
	sort.Strings(modifiedFiles)
	return
}

// FormatFileOperations 格式化文件操作为文本。
func FormatFileOperations(readFiles, modifiedFiles []string) string {
	var sections []string
	if len(readFiles) > 0 {
		sections = append(sections, fmt.Sprintf("<read-files>\n%s\n</read-files>", strings.Join(readFiles, "\n")))
	}
	if len(modifiedFiles) > 0 {
		sections = append(sections, fmt.Sprintf("<modified-files>\n%s\n</modified-files>", strings.Join(modifiedFiles, "\n")))
	}
	if len(sections) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(sections, "\n\n")
}

// ---------- 自适应分块 ----------

// EstimateMessagesTokens 估算消息 token 数。
func EstimateMessagesTokens(messages []AgentMessage) int {
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content) + len(msg.ToolName) + 20
	}
	return totalChars / 4 // 粗略估算
}

// ComputeAdaptiveChunkRatio 计算自适应分块比率。
func ComputeAdaptiveChunkRatio(messages []AgentMessage, contextWindowTokens int) float64 {
	if contextWindowTokens <= 0 || len(messages) == 0 {
		return BaseChunkRatio
	}

	tokens := EstimateMessagesTokens(messages)
	ratio := float64(tokens) / float64(contextWindowTokens)

	if ratio <= 0.5 {
		return BaseChunkRatio
	}
	// 随消息大小线性缩减
	adjusted := BaseChunkRatio * (1.0 - (ratio-0.5)*0.5)
	if adjusted < MinChunkRatio {
		return MinChunkRatio
	}
	return adjusted
}

// IsOversizedForSummary 检查是否超过摘要大小限制。
func IsOversizedForSummary(tokens, contextWindowTokens int) bool {
	if contextWindowTokens <= 0 {
		return false
	}
	return float64(tokens) > float64(contextWindowTokens)*SafetyMargin
}

// ---------- 入口 ----------

// BuildCompactionFallback 构建默认 fallback compaction 结果。
func BuildCompactionFallback(prep CompactionPreparation) CompactionResult {
	readFiles, modifiedFiles := ComputeFileLists(prep.FileOps)
	failures := CollectToolFailures(append(prep.MessageToSummarize, prep.TurnPrefixMessages...))
	failureSection := FormatToolFailuresSection(failures)
	fileOpsSummary := FormatFileOperations(readFiles, modifiedFiles)

	return CompactionResult{
		Summary:          FallbackSummary + failureSection + fileOpsSummary,
		FirstKeptEntryID: prep.FirstKeptEntryID,
		TokensBefore:     prep.TokensBefore,
		ReadFiles:        readFiles,
		ModifiedFiles:    modifiedFiles,
	}
}
