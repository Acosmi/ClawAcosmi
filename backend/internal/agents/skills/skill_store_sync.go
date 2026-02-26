package skills

// skill_store_sync.go — 远程技能下载 → 本地 docs/skills/synced/ 同步
//
// 流程: Download ZIP → 解压 → 提取/转换 SKILL.md → 写入 synced/{key}/SKILL.md

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PullResult 单个技能拉取结果。
type PullResult struct {
	SkillName string `json:"skillName"`
	SkillID   string `json:"skillId"`
	Dir       string `json:"dir"`   // 写入的本地目录
	IsNew     bool   `json:"isNew"` // true=新增, false=更新
}

// maxZipSize 下载 ZIP 最大允许大小 (10 MB)。
const maxZipSize = 10 * 1024 * 1024

// PullSkillToLocal 从远程下载单个技能并写入本地 synced/ 目录。
// docsSkillsDir 是 docs/skills/ 的绝对路径。
func PullSkillToLocal(client *SkillStoreClient, skillID string, docsSkillsDir string) (*PullResult, error) {
	if !client.Available() {
		return nil, fmt.Errorf("skill store client not configured")
	}

	// 1. 下载 ZIP
	zipData, _, err := client.Download(skillID)
	if err != nil {
		return nil, fmt.Errorf("download skill %s: %w", skillID, err)
	}

	// 安全: ZIP 大小限制
	if len(zipData) > maxZipSize {
		return nil, fmt.Errorf("skill %s: zip too large (%d bytes, max %d)", skillID, len(zipData), maxZipSize)
	}

	// 2. 解析 ZIP 内容
	skillContent, skillKey, err := extractSkillFromZip(zipData, skillID)
	if err != nil {
		return nil, fmt.Errorf("extract skill %s: %w", skillID, err)
	}

	// 3. 写入 docs/skills/synced/{key}/SKILL.md
	syncedDir := filepath.Join(docsSkillsDir, "synced")
	targetDir := filepath.Join(syncedDir, skillKey)

	isNew := true
	if _, statErr := os.Stat(filepath.Join(targetDir, "SKILL.md")); statErr == nil {
		isNew = false
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("create synced dir %s: %w", targetDir, err)
	}

	skillPath := filepath.Join(targetDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
		return nil, fmt.Errorf("write SKILL.md to %s: %w", skillPath, err)
	}

	return &PullResult{
		SkillName: skillKey,
		SkillID:   skillID,
		Dir:       targetDir,
		IsNew:     isNew,
	}, nil
}

// BatchPull 批量拉取技能。逐个下载，收集结果和错误。
func BatchPull(client *SkillStoreClient, skillIDs []string, docsSkillsDir string) ([]PullResult, []error) {
	var results []PullResult
	var errs []error

	for _, id := range skillIDs {
		result, err := PullSkillToLocal(client, id, docsSkillsDir)
		if err != nil {
			errs = append(errs, fmt.Errorf("skill %s: %w", id, err))
			continue
		}
		results = append(results, *result)
	}

	return results, errs
}

// extractSkillFromZip 从 ZIP 中提取技能内容。
// 返回: SKILL.md 内容, 技能 key, error。
//
// 支持两种格式:
// 1. 直接包含 SKILL.md → 直接使用
// 2. nexus-v4 格式 manifest.json → 转换为 SKILL.md
func extractSkillFromZip(zipData []byte, fallbackID string) (string, string, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return "", "", fmt.Errorf("open zip: %w", err)
	}

	var skillMD string
	var manifestData []byte
	skillKey := fallbackID

	for _, f := range reader.File {
		// 安全: 拒绝包含路径遍历的 ZIP 条目
		cleanPath := filepath.Clean(f.Name)
		if !filepath.IsLocal(cleanPath) {
			return "", "", fmt.Errorf("zip contains unsafe path: %s", f.Name)
		}

		// 安全: 单个文件大小限制 (5 MB)
		if f.UncompressedSize64 > 5*1024*1024 {
			return "", "", fmt.Errorf("zip entry too large: %s (%d bytes)", f.Name, f.UncompressedSize64)
		}

		name := filepath.Base(cleanPath)

		if name == "SKILL.md" {
			rc, err := f.Open()
			if err != nil {
				return "", "", fmt.Errorf("open SKILL.md in zip: %w", err)
			}
			data, err := io.ReadAll(io.LimitReader(rc, 5*1024*1024+1))
			rc.Close()
			if err != nil {
				return "", "", fmt.Errorf("read SKILL.md in zip: %w", err)
			}
			skillMD = string(data)

			// 从目录名推导 key（使用已清理的路径）
			dir := filepath.Dir(cleanPath)
			if dir != "." && dir != "" {
				skillKey = filepath.Base(dir)
			}
		}

		if name == "manifest.json" {
			rc, err := f.Open()
			if err != nil {
				return "", "", fmt.Errorf("open manifest.json in zip: %w", err)
			}
			manifestData, err = io.ReadAll(io.LimitReader(rc, 1*1024*1024))
			rc.Close()
			if err != nil {
				return "", "", fmt.Errorf("read manifest.json in zip: %w", err)
			}
		}
	}

	// 优先使用已有的 SKILL.md
	if skillMD != "" {
		return skillMD, sanitizeKey(skillKey), nil
	}

	// 从 manifest.json 转换
	if manifestData != nil {
		content, key, err := convertManifestToSkillMD(manifestData)
		if err != nil {
			return "", "", fmt.Errorf("convert manifest: %w", err)
		}
		if key != "" {
			skillKey = key
		}
		return content, sanitizeKey(skillKey), nil
	}

	return "", "", fmt.Errorf("zip contains neither SKILL.md nor manifest.json")
}

// nexusManifest nexus-v4 技能 manifest.json 结构（精简）。
type nexusManifest struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Content     string `json:"content"` // 技能 prompt 内容
	Icon        string `json:"icon"`
}

// convertManifestToSkillMD 将 nexus-v4 manifest.json 转换为 SKILL.md 格式。
func convertManifestToSkillMD(data []byte) (string, string, error) {
	var m nexusManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return "", "", fmt.Errorf("parse manifest.json: %w", err)
	}

	if m.Name == "" && m.Key == "" {
		return "", "", fmt.Errorf("manifest.json missing name and key")
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	if m.Name != "" {
		sb.WriteString(fmt.Sprintf("name: %s\n", m.Name))
	}
	if m.Description != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", m.Description))
	}
	if m.Category != "" {
		sb.WriteString(fmt.Sprintf("category: %s\n", m.Category))
	}
	if m.Version != "" {
		sb.WriteString(fmt.Sprintf("version: %s\n", m.Version))
	}
	if m.Author != "" {
		sb.WriteString(fmt.Sprintf("author: %s\n", m.Author))
	}
	if m.Icon != "" {
		sb.WriteString(fmt.Sprintf("icon: %s\n", m.Icon))
	}
	sb.WriteString("source: synced\n")
	sb.WriteString("---\n\n")

	if m.Content != "" {
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	} else if m.Description != "" {
		sb.WriteString(m.Description)
		sb.WriteString("\n")
	}

	key := m.Key
	if key == "" {
		key = sanitizeKey(m.Name)
	}

	return sb.String(), key, nil
}

// sanitizeKey 将技能名称转为安全的目录名。
func sanitizeKey(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	// 只保留字母、数字、连字符
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			sb.WriteRune(r)
		} else if r == ' ' || r == '_' {
			sb.WriteRune('-')
		}
	}
	result := sb.String()
	// 合并连续连字符
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}
