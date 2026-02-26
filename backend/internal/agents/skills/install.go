package skills

// install.go — 技能安装 / 卸载 / 命令规格
// 对应 TS: agents/skills/workspace.ts (部分) + serialize.ts + plugin-skills.ts
//
// 提供 InstallSkill / UninstallSkill / CheckSkillStatus /
// BuildWorkspaceSkillCommandSpecs / SyncSkillsToWorkspace。

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// SkillStatus 技能状态。
type SkillStatus struct {
	Installed   bool   `json:"installed"`
	Name        string `json:"name"`
	Dir         string `json:"dir,omitempty"`
	Description string `json:"description,omitempty"`
	HasContent  bool   `json:"hasContent"`
}

// InstallSkill 安装技能到工作区。
// 对应 TS: workspace.ts → installSkill（概念对应）
func InstallSkill(workspaceDir string, name string, content string) error {
	skillDir := filepath.Join(workspaceDir, ".agent", "skills", name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return err
	}
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if content == "" {
		content = "---\ndescription: " + name + "\n---\n\n# " + name + "\n"
	}
	if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
		return err
	}
	slog.Info("skill installed", "name", name, "dir", skillDir)
	return nil
}

// UninstallSkill 卸载技能。
func UninstallSkill(workspaceDir string, name string) error {
	skillDir := filepath.Join(workspaceDir, ".agent", "skills", name)
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return nil // 已不存在
	}
	if err := os.RemoveAll(skillDir); err != nil {
		return err
	}
	slog.Info("skill uninstalled", "name", name, "dir", skillDir)
	return nil
}

// CheckSkillStatus 检查技能状态。
func CheckSkillStatus(workspaceDir string, name string) SkillStatus {
	skillDir := filepath.Join(workspaceDir, ".agent", "skills", name)
	skillFile := filepath.Join(skillDir, "SKILL.md")

	content, err := os.ReadFile(skillFile)
	if err != nil {
		return SkillStatus{Name: name, Installed: false}
	}

	desc := extractDescription(string(content))
	return SkillStatus{
		Installed:   true,
		Name:        name,
		Dir:         skillDir,
		Description: desc,
		HasContent:  len(content) > 0,
	}
}

// BuildWorkspaceSkillCommandSpecs 构建工作区技能命令规格列表。
// 对应 TS: serialize.ts → buildWorkspaceSkillCommandSpecs（概念对应）
func BuildWorkspaceSkillCommandSpecs(entries []SkillEntry) []SkillCommandSpec {
	var specs []SkillCommandSpec
	for _, entry := range entries {
		if !entry.Enabled {
			continue
		}
		// 解析 frontmatter 中的 commands
		fm := ParseFrontmatter(entry.Skill.Content)
		if cmdRaw, ok := fm["commands"]; ok && cmdRaw != "" {
			var commands []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			if err := json.Unmarshal([]byte(cmdRaw), &commands); err == nil {
				for _, cmd := range commands {
					if cmd.Name == "" {
						continue
					}
					specs = append(specs, SkillCommandSpec{
						Name:        cmd.Name,
						SkillName:   entry.Skill.Name,
						Description: cmd.Description,
					})
				}
			}
		}

		// 隐式 slash 命令（/skillname）
		if entry.Skill.Description != "" {
			specs = append(specs, SkillCommandSpec{
				Name:        entry.Skill.Name,
				SkillName:   entry.Skill.Name,
				Description: entry.Skill.Description,
			})
		}
	}
	return specs
}

// SyncSkillsToWorkspace 同步技能到工作区。
// 对应 TS: workspace.ts → syncSkillsToWorkspace（概念对应）
func SyncSkillsToWorkspace(workspaceDir string, managed []Skill) error {
	managedDir := filepath.Join(workspaceDir, ".agent", "skills-managed")
	if err := os.MkdirAll(managedDir, 0755); err != nil {
		return err
	}

	// 清理不再需要的 managed skills
	existingEntries, _ := os.ReadDir(managedDir)
	managedNames := make(map[string]bool)
	for _, s := range managed {
		managedNames[s.Name] = true
	}
	for _, de := range existingEntries {
		if de.IsDir() && !managedNames[de.Name()] {
			os.RemoveAll(filepath.Join(managedDir, de.Name()))
		}
	}

	// 写入/更新 managed skills
	for _, s := range managed {
		skillDir := filepath.Join(managedDir, s.Name)
		os.MkdirAll(skillDir, 0755)
		content := s.Content
		if content == "" {
			content = "---\ndescription: " + s.Description + "\n---\n"
		}
		skillFile := filepath.Join(skillDir, "SKILL.md")

		// 只在内容变更时写入
		existing, err := os.ReadFile(skillFile)
		if err != nil || strings.TrimSpace(string(existing)) != strings.TrimSpace(content) {
			os.WriteFile(skillFile, []byte(content), 0644)
		}
	}

	return nil
}
