package skills

// skill_distributor.go — 技能 VFS 分级 + Qdrant 索引入库。
//
// 将 SKILL.md 解析为 L0/L1/L2 三级内容写入 VFS _system/skills/，
// 同时在 Qdrant sys_skills collection 中建立 payload 索引。

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/openacosmi/claw-acismi/internal/memory/uhms"
)

// SkillDistributeResult 技能分级结果。
type SkillDistributeResult struct {
	Indexed        int           `json:"indexed"`         // VFS 新写入数
	Skipped        int           `json:"skipped"`         // VFS 增量跳过数（content_hash 未变）
	SearchUpserted int           `json:"search_upserted"` // 搜索引擎 upsert 成功数
	Errors         []string      `json:"errors,omitempty"`
	Duration       time.Duration `json:"duration"`
}

// DistributeSkills 将技能解析为 L0/L1/L2 写入 VFS，并在 Qdrant 中建立索引。
// vfs 用于写入分级内容，vectorIndex 用于建立检索索引（可选，nil 时跳过索引）。
func DistributeSkills(ctx context.Context, vfs uhms.VFS, vectorIndex uhms.VectorIndex, entries []SkillEntry) (*SkillDistributeResult, error) {
	start := time.Now()
	result := &SkillDistributeResult{}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		vfsSkipped, searchUpserted, err := distributeOneSkill(ctx, vfs, vectorIndex, entry)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", entry.Skill.Name, err))
			slog.Warn("skill_distributor: failed to distribute skill",
				"name", entry.Skill.Name, "error", err)
			continue
		}
		if vfsSkipped {
			result.Skipped++
		} else {
			result.Indexed++
		}
		if searchUpserted {
			result.SearchUpserted++
		}
	}

	result.Duration = time.Since(start)
	slog.Info("skill_distributor: distribute complete",
		"vfs_written", result.Indexed, "vfs_skipped", result.Skipped,
		"search_upserted", result.SearchUpserted,
		"errors", len(result.Errors), "duration", result.Duration)
	return result, nil
}

// distributeOneSkill 处理单个技能的 VFS 分级 + Qdrant 索引。
// 返回 (vfsSkipped, searchUpserted, error):
//   - vfsSkipped: content_hash 未变则 VFS 写入 skip
//   - searchUpserted: Qdrant payload upsert 成功
//
// 设计说明: VFS 写入有增量跳过（content_hash），但 Qdrant 索引始终执行。
// 原因: Qdrant Segment 重启后内存集合为空（build_segment 创建全新 segment），
// 如果 content_hash 短路也跳过 Qdrant upsert，重启后搜索引擎将永远为空。
// Qdrant upsert 是 idempotent 操作，重复写入无副作用。
func distributeOneSkill(ctx context.Context, vfs uhms.VFS, vectorIndex uhms.VectorIndex, entry SkillEntry) (bool, bool, error) {
	name := entry.Skill.Name
	category := ResolveSkillCategory(entry)
	contentHash := computeContentHash(entry.Skill.Content)
	tags := extractTags(entry.Skill.Content)

	// 增量检查: content_hash 未变则跳过 VFS 写入
	vfsUpToDate := false
	if meta, err := vfs.ReadSystemMeta("skills", category, name); err == nil {
		if existingHash, ok := meta["content_hash"].(string); ok && existingHash == contentHash {
			vfsUpToDate = true
		}
	}

	// L0 摘要始终生成（Qdrant payload 需要 l0_abstract 字段）
	l0 := generateSkillL0(entry)

	// VFS 写入（增量: content_hash 变化时才写）
	if !vfsUpToDate {
		l1 := generateSkillL1(entry)
		l2 := generateSkillL2(entry)

		meta := map[string]interface{}{
			"name":           name,
			"category":       category,
			"description":    entry.Skill.Description,
			"content_hash":   contentHash,
			"distributed":    true,
			"distributed_at": time.Now().UTC().Format(time.RFC3339),
			"source_dir":     entry.Skill.Dir,
		}
		if tags != "" {
			meta["tags"] = tags
		}

		if err := vfs.WriteSystemEntry("skills", category, name, l0, l1, l2, meta); err != nil {
			return false, false, fmt.Errorf("write VFS: %w", err)
		}
	}

	// Qdrant 索引 — 始终执行（idempotent; Segment 重启后内存集合为空，需要重新填充）
	searchUpserted := false
	if vectorIndex != nil {
		vfsPath := fmt.Sprintf("_system/skills/%s/%s", category, name)
		payload := map[string]interface{}{
			"name":         name,
			"category":     category,
			"description":  entry.Skill.Description,
			"tags":         tags,
			"vfs_path":     vfsPath,
			"content_hash": contentHash,
			"distributed":  true,
			"l0_abstract":  l0,
		}

		id := deterministicSkillID(name)

		type payloadUpserter interface {
			UpsertPayload(ctx context.Context, collection, id string, payload map[string]interface{}) error
		}
		if pu, ok := vectorIndex.(payloadUpserter); ok {
			if err := pu.UpsertPayload(ctx, "sys_skills", id, payload); err != nil {
				slog.Warn("skill_distributor: search engine upsert failed (non-fatal)",
					"name", name, "error", err)
			} else {
				searchUpserted = true
			}
		} else {
			slog.Warn("skill_distributor: vectorIndex does not support UpsertPayload (non-fatal)",
				"name", name)
		}
	}

	return vfsUpToDate, searchUpserted, nil
}

// generateSkillL0 生成技能 L0 摘要 (~100 tokens)。
// 格式: {emoji} {name}: {description} [tags: {tags}]
func generateSkillL0(entry SkillEntry) string {
	var sb strings.Builder

	// emoji
	if entry.Metadata != nil && entry.Metadata.Emoji != "" {
		sb.WriteString(entry.Metadata.Emoji)
		sb.WriteString(" ")
	}

	sb.WriteString(entry.Skill.Name)

	if entry.Skill.Description != "" {
		sb.WriteString(": ")
		desc := entry.Skill.Description
		if len(desc) > 150 {
			desc = desc[:147] + "..."
		}
		sb.WriteString(desc)
	}

	tags := extractTags(entry.Skill.Content)
	if tags != "" {
		sb.WriteString(" [tags: ")
		sb.WriteString(tags)
		sb.WriteString("]")
	}

	return sb.String()
}

// generateSkillL1 生成技能 L1 概览 (~2K tokens)。
// 包含 frontmatter 完整信息 + SKILL.md 前 2K tokens。
func generateSkillL1(entry SkillEntry) string {
	content := entry.Skill.Content
	// 截取前 ~8000 字符 (~2K tokens)
	runes := []rune(content)
	if len(runes) > 8000 {
		return string(runes[:8000]) + "\n[... truncated for L1 overview]"
	}
	return content
}

// generateSkillL2 生成技能 L2 完整内容。
func generateSkillL2(entry SkillEntry) string {
	return entry.Skill.Content
}

// computeContentHash 计算内容 MD5 哈希。
func computeContentHash(content string) string {
	h := md5.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}

// deterministicSkillID 生成确定性技能 ID（基于 name 的 MD5）。
func deterministicSkillID(name string) string {
	h := md5.New()
	h.Write([]byte("skill:" + name))
	hash := hex.EncodeToString(h.Sum(nil))
	// 格式化为 UUID-like
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hash[:8], hash[8:12], hash[12:16], hash[16:20], hash[20:32])
}

// ResolveSkillCategory 推断技能分类。
// 从 Dir 路径提取最近的分类目录名 (如 tools, providers, general, official)。
func ResolveSkillCategory(entry SkillEntry) string {
	dir := entry.Skill.Dir
	if dir == "" {
		return "general"
	}
	// 从路径中提取: .../docs/skills/{category}/{name}/
	// 或: .../skills/{name}/
	parts := strings.Split(dir, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == entry.Skill.Name && i > 0 {
			parent := parts[i-1]
			if parent != "skills" && parent != ".agent" {
				return parent
			}
		}
	}
	return "general"
}

// extractTags 从 SKILL.md frontmatter 提取 tags。
func extractTags(content string) string {
	if !strings.HasPrefix(content, "---") {
		return ""
	}
	end := strings.Index(content[3:], "---")
	if end == -1 {
		return ""
	}
	frontmatter := content[3 : 3+end]
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "tags:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "tags:"))
		}
	}
	return ""
}

// CollectDistributedCategories 收集所有已分级技能的分类名称。
func CollectDistributedCategories(entries []SkillEntry) []string {
	seen := make(map[string]bool)
	for _, e := range entries {
		cat := ResolveSkillCategory(e)
		seen[cat] = true
	}
	cats := make([]string, 0, len(seen))
	for cat := range seen {
		cats = append(cats, cat)
	}
	return cats
}
