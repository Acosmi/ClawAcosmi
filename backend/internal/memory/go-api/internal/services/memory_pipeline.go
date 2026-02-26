// Package services — Two-Stage Memory Pipeline (Mem0-style Extract→Compare→Update).
// Mirrors Python services/memory_pipeline.py — dedup before add to keep knowledge base clean.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// MemoryAction represents the deduplication decision.
type MemoryAction string

const (
	ActionAdd    MemoryAction = "add"    // New unique memory
	ActionUpdate MemoryAction = "update" // Existing memory should be updated
	ActionDelete MemoryAction = "delete" // Existing memory is obsolete
	ActionNoop   MemoryAction = "noop"   // Duplicate, no action needed
)

// DedupMetadata contains info for applying the action.
type DedupMetadata struct {
	TargetMemoryID string `json:"target_memory_id,omitempty"`
	MergedContent  string `json:"merged_content,omitempty"`
	Reason         string `json:"reason"`
}

// Dedup prompt template
const dedupPrompt = `You are a memory deduplication agent. Given a NEW memory and a list of EXISTING similar memories, decide what action to take.

NEW MEMORY:
%s

EXISTING SIMILAR MEMORIES:
%s

Decide ONE action:
- "add": The new memory is genuinely new information, not covered by existing memories.
- "update": The new memory contains updated/additional information about an existing memory. Specify which existing memory ID to update and the merged content.
- "delete": The new memory contradicts an existing memory, making it obsolete. Specify which existing memory ID to delete.
- "noop": The new memory is a duplicate of an existing memory. No action needed.

Respond in JSON format:
{
  "action": "add" | "update" | "delete" | "noop",
  "target_memory_id": "uuid-of-existing-memory-if-applicable",
  "merged_content": "merged content if action is update",
  "reason": "brief explanation"
}`

// RunDedupPipeline executes the two-stage deduplication pipeline.
//
// Stage 1: Search for similar existing memories via vector store.
// Stage 2: Ask LLM to decide action.
//
// Returns (action, metadata).
func RunDedupPipeline(
	ctx context.Context,
	newContent string,
	userID string,
	memoryType string,
	vectorStore *VectorStoreService,
	llm LLMProvider,
	similarityThreshold float64,
	maxCandidates int,
) (MemoryAction, DedupMetadata) {
	if similarityThreshold <= 0 {
		similarityThreshold = 0.75
	}
	if maxCandidates <= 0 {
		maxCandidates = 5
	}

	// Stage 1: Search for similar memories
	var memTypes []string
	if memoryType != "" {
		memTypes = []string{memoryType}
	}
	candidates, err := vectorStore.HybridSearch(ctx, newContent, userID, maxCandidates, memTypes, 0, "", nil, nil, nil, nil)
	if err != nil {
		slog.Warn("Dedup search failed, defaulting to ADD", "error", err)
		return ActionAdd, DedupMetadata{Reason: "search_failed"}
	}

	if len(candidates) == 0 {
		return ActionAdd, DedupMetadata{Reason: "no_similar_memories"}
	}

	// Filter by similarity threshold
	var similar []VectorSearchResult
	for _, c := range candidates {
		if c.Score >= similarityThreshold {
			similar = append(similar, c)
		}
	}
	if len(similar) == 0 {
		return ActionAdd, DedupMetadata{Reason: "below_similarity_threshold"}
	}

	// Stage 2: Ask LLM to decide
	existingText := ""
	for _, c := range similar {
		existingText += fmt.Sprintf("- ID: %s, Content: %s, Score: %.2f\n", c.MemoryID.String(), c.Content, c.Score)
	}

	prompt := fmt.Sprintf(dedupPrompt, newContent, existingText)

	response, err := llm.Generate(ctx, prompt)
	if err != nil {
		slog.Warn("Dedup LLM decision failed, defaulting to NOOP (safe)", "error", err)
		return ActionNoop, DedupMetadata{Reason: fmt.Sprintf("llm_decision_failed: %v", err)}
	}

	decision, err := ParseLLMJSON(response)
	if err != nil {
		slog.Warn("Dedup JSON parse failed, defaulting to NOOP (safe)", "error", err)
		return ActionNoop, DedupMetadata{Reason: "json_parse_failed"}
	}

	actionStr, _ := decision["action"].(string)
	action := MemoryAction(actionStr)
	if action == "" {
		action = ActionNoop // 安全默认值：不插入，避免重复
	}

	meta := DedupMetadata{
		Reason: stringOrDefault(decision, "reason", ""),
	}
	if tid, ok := decision["target_memory_id"].(string); ok {
		meta.TargetMemoryID = tid
	}
	if mc, ok := decision["merged_content"].(string); ok {
		meta.MergedContent = mc
	}

	slog.Info("Dedup pipeline decision", "action", action, "reason", meta.Reason)
	return action, meta
}

// ApplyDedupAction applies the dedup action to the database.
// When action is UPDATE, also syncs the new embedding to VectorStore.
// Returns:
//   - "" if action is ADD (caller should proceed with normal add)
//   - memory_id if action is UPDATE (memory was updated in-place)
//   - "skipped" if action is NOOP or DELETE was performed
func ApplyDedupAction(
	ctx context.Context,
	db *gorm.DB,
	action MemoryAction,
	meta DedupMetadata,
	vectorStore *VectorStoreService,
) string {
	if action == ActionAdd {
		return "" // Caller proceeds with normal add
	}

	if action == ActionNoop {
		slog.Info("Dedup: Skipping duplicate memory")
		return "skipped"
	}

	if meta.TargetMemoryID == "" {
		slog.Warn("Dedup: No target_memory_id, defaulting to ADD")
		return ""
	}

	targetUUID, err := uuid.Parse(meta.TargetMemoryID)
	if err != nil {
		slog.Warn("Dedup: Invalid target_memory_id", "id", meta.TargetMemoryID)
		return ""
	}

	var memory models.Memory
	if err := db.First(&memory, "id = ?", targetUUID).Error; err != nil {
		slog.Warn("Dedup: Target memory not found", "id", meta.TargetMemoryID)
		return ""
	}

	switch action {
	case ActionUpdate:
		if meta.MergedContent != "" {
			if err := db.Model(&memory).Update("content", meta.MergedContent).Error; err != nil {
				slog.Error("Dedup: Failed to update memory", "id", meta.TargetMemoryID, "error", err)
				return ""
			}
			// Sync updated embedding to VectorStore
			if vectorStore != nil {
				if err := vectorStore.AddMemory(
					ctx, targetUUID, meta.MergedContent,
					memory.UserID, string(memory.MemoryType),
					memory.ImportanceScore, nil,
				); err != nil {
					slog.Warn("Dedup: Failed to sync VectorStore after UPDATE", "id", meta.TargetMemoryID, "error", err)
				}
			}
			slog.Info("Dedup: Updated memory with merged content", "id", meta.TargetMemoryID)
		}
		return meta.TargetMemoryID

	case ActionDelete:
		if err := db.Delete(&memory).Error; err != nil {
			slog.Error("Dedup: Failed to delete memory", "id", meta.TargetMemoryID, "error", err)
			return ""
		}
		slog.Info("Dedup: Deleted obsolete memory", "id", meta.TargetMemoryID)
		return "skipped"
	}

	return ""
}

// --- Helper ---

func stringOrDefault(m map[string]any, key, def string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return def
}

// ============================================================================
// Bi-temporal: 事件时间抽取 (Event Time Extraction)
// ============================================================================

// eventTimePrompt LLM 事件时间抽取 Prompt 模板。
// 注入当前日期以帮助 LLM 解析 "上周"、"去年" 等相对时间表达。
const eventTimePrompt = `You are a temporal information extractor. Given text, extract when the described event(s) occurred (NOT when the text was written).

Current date: %s

TEXT:
%s

Rules:
- Extract the MOST SPECIFIC event time mentioned in the text.
- Use ISO 8601 format: YYYY-MM-DDTHH:MM:SSZ (or YYYY-MM-DD if only date is known).
- For relative expressions (e.g. "last week", "yesterday"), calculate the exact date based on current date.
- If MULTIPLE events with different times are mentioned, extract the PRIMARY/MOST IMPORTANT one.
- If NO event time can be determined (e.g. general statements, opinions), return null.

Respond in JSON:
{
  "event_time": "2025-06-15T00:00:00Z" or null,
  "confidence": "high" | "medium" | "low",
  "reasoning": "brief explanation"
}`

// ExtractEventTime 使用 LLM 从文本中抽取事件发生时间。
// 对标 Zep/Graphiti 的时序抽取能力。
//
// 返回值:
//   - *time.Time: 提取到的事件时间（nil 表示无法提取或文本无时间信息）
//   - error: 仅在系统级错误时返回（LLM 超时等），业务上的"无法提取"返回 nil, nil
func ExtractEventTime(ctx context.Context, llm LLMProvider, content string) (*time.Time, error) {
	if llm == nil {
		return nil, nil
	}

	now := time.Now().Format("2006-01-02")
	prompt := fmt.Sprintf(eventTimePrompt, now, content)

	response, err := llm.Generate(ctx, prompt)
	if err != nil {
		slog.Warn("事件时间抽取 LLM 调用失败，跳过", "error", err)
		return nil, nil // 非致命：LLM 失败不阻塞记忆创建
	}

	parsed, err := ParseLLMJSON(response)
	if err != nil {
		slog.Warn("事件时间抽取 JSON 解析失败", "error", err)
		return nil, nil
	}

	// 检查 event_time 字段
	eventTimeRaw, ok := parsed["event_time"]
	if !ok || eventTimeRaw == nil {
		slog.Debug("文本中未检测到事件时间")
		return nil, nil
	}

	eventTimeStr, ok := eventTimeRaw.(string)
	if !ok || eventTimeStr == "" {
		return nil, nil
	}

	// 尝试多种 ISO 8601 格式解析
	t, parseErr := parseFlexibleTime(eventTimeStr)
	if parseErr != nil {
		slog.Warn("事件时间解析失败", "raw", eventTimeStr, "error", parseErr)
		return nil, nil
	}

	confidence := stringOrDefault(parsed, "confidence", "low")
	slog.Info("事件时间抽取成功",
		"event_time", t.Format(time.RFC3339),
		"confidence", confidence,
	)
	return &t, nil
}

// parseFlexibleTime 尝试多种常用时间格式解析字符串。
func parseFlexibleTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	formats := []string{
		time.RFC3339,           // 2006-01-02T15:04:05Z07:00
		"2006-01-02T15:04:05Z", // UTC 显式
		"2006-01-02T15:04:05",  // 无时区
		"2006-01-02",           // 纯日期
	}
	for _, fmt := range formats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("无法解析时间: %s", s)
}
