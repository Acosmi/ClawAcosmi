package skills

// bundled_dir.go — 内置技能目录解析
// 对应 TS: agents/skills/bundled-dir.ts (91L)
//
// 提供 ResolveBundledSkillsDir — 多策略定位 bundled skills 目录：
//   1. 环境变量覆盖 (OPENACOSMI_BUNDLED_SKILLS_DIR)
//   2. 可执行文件同级 skills/ 目录
//   3. 工作目录向上遍历（最多 6 层）

import (
	"os"
	"path/filepath"
	"strings"
)

// looksLikeSkillsDir 检查目录是否看起来像技能目录。
// 对应 TS: bundled-dir.ts → looksLikeSkillsDir
//
// 判据：目录中存在 .md 文件或含 SKILL.md 的子目录。
func looksLikeSkillsDir(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			return true
		}
		if entry.IsDir() {
			skillMd := filepath.Join(dir, entry.Name(), "SKILL.md")
			if _, err := os.Stat(skillMd); err == nil {
				return true
			}
		}
	}
	return false
}

// ResolveBundledSkillsDir 解析捆绑技能目录。
// 对应 TS: bundled-dir.ts → resolveBundledSkillsDir
//
// 多策略定位：
//  1. 环境变量 OPENACOSMI_BUNDLED_SKILLS_DIR
//  2. execPath 同级 skills/ 目录
//  3. 当前可执行文件同级 skills/ 目录
//  4. 当前工作目录向上遍历（最多 6 层），寻找 skills/ 子目录
func ResolveBundledSkillsDir(execPath string) string {
	// 1) 环境变量覆盖
	override := strings.TrimSpace(os.Getenv("OPENACOSMI_BUNDLED_SKILLS_DIR"))
	if override != "" {
		return override
	}

	// 2) 指定 execPath 同级 skills/ 目录
	if execPath != "" {
		sibling := filepath.Join(filepath.Dir(execPath), "skills")
		if info, err := os.Stat(sibling); err == nil && info.IsDir() {
			return sibling
		}
	}

	// 3) 当前可执行文件路径
	if ep, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(ep), "skills")
		if info, err := os.Stat(sibling); err == nil && info.IsDir() {
			return sibling
		}
	}

	// 4) 当前工作目录向上遍历（TS: moduleDir 向上 6 层 + looksLikeSkillsDir 验证）
	if cwd, err := os.Getwd(); err == nil {
		current := cwd
		for depth := 0; depth < 6; depth++ {
			candidate := filepath.Join(current, "skills")
			if looksLikeSkillsDir(candidate) {
				return candidate
			}
			next := filepath.Dir(current)
			if next == current {
				break
			}
			current = next
		}
	}

	return ""
}
