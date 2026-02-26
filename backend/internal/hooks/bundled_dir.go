package hooks

import (
	"os"
	"path/filepath"
)

// ============================================================================
// Bundled hooks 目录解析
// 对应 TS: bundled-dir.ts
// ============================================================================

// ResolveBundledHooksDir 解析 bundled hooks 目录
// 对应 TS: bundled-dir.ts resolveBundledHooksDir
//
// 查找顺序：
// 1. 环境变量 OPENACOSMI_BUNDLED_HOOKS_DIR
// 2. 可执行文件同级 hooks/bundled/
// 3. 相对于工作目录的 hooks/bundled/
func ResolveBundledHooksDir() string {
	// 1. 环境变量 override
	if override := os.Getenv("OPENACOSMI_BUNDLED_HOOKS_DIR"); override != "" {
		return override
	}

	// 2. 可执行文件同级
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		sibling := filepath.Join(execDir, "hooks", "bundled")
		if info, err := os.Stat(sibling); err == nil && info.IsDir() {
			return sibling
		}
	}

	// 3. 工作目录
	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, "hooks", "bundled")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	return ""
}
