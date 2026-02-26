package tts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TS 对照: tts/tts.ts L305-466 (偏好管理)

// ---------- 偏好路径 ----------

// ResolveTtsPrefsPath 解析 TTS 偏好文件路径。
// TS 对照: tts.ts L305-314
func ResolveTtsPrefsPath(config ResolvedTtsConfig) string {
	if strings.TrimSpace(config.PrefsPath) != "" {
		return resolveUserPath(config.PrefsPath)
	}
	envPath := strings.TrimSpace(os.Getenv("OPENACOSMI_TTS_PREFS"))
	if envPath != "" {
		return resolveUserPath(envPath)
	}
	return filepath.Join(configDir(), "settings", "tts.json")
}

// ---------- 偏好读写 ----------

// readPrefs 读取偏好文件。
// TS 对照: tts.ts L368-377
func readPrefs(prefsPath string) TtsUserPrefs {
	data, err := os.ReadFile(prefsPath)
	if err != nil {
		return TtsUserPrefs{}
	}
	var prefs TtsUserPrefs
	if err := json.Unmarshal(data, &prefs); err != nil {
		return TtsUserPrefs{}
	}
	return prefs
}

// updatePrefs 原子更新偏好文件。
// TS 对照: tts.ts L394-399
func updatePrefs(prefsPath string, update func(*TtsUserPrefs)) error {
	prefs := readPrefs(prefsPath)
	update(&prefs)

	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化偏好失败: %w", err)
	}

	dir := filepath.Dir(prefsPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建偏好目录失败: %w", err)
	}

	// 原子写入
	tmpPath := fmt.Sprintf("%s.tmp.%d", prefsPath, os.Getpid())
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, prefsPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("原子重命名失败: %w", err)
	}
	return nil
}

// ---------- 自动模式 ----------

// ResolveTtsAutoMode 解析实际 TTS 自动模式。
// 优先级: session > prefs > config。
// TS 对照: tts.ts L327-341
func ResolveTtsAutoMode(config ResolvedTtsConfig, prefsPath, sessionAuto string) TtsAutoMode {
	if sa := NormalizeTtsAutoMode(sessionAuto); sa != "" {
		return sa
	}
	prefs := readPrefs(prefsPath)
	if prefs.Tts != nil {
		if pa := NormalizeTtsAutoMode(string(prefs.Tts.Auto)); pa != "" {
			return pa
		}
		if prefs.Tts.Enabled != nil {
			if *prefs.Tts.Enabled {
				return AutoAlways
			}
			return AutoOff
		}
	}
	return config.Auto
}

// IsTtsEnabled 判断 TTS 是否启用。
// TS 对照: tts.ts L401-407
func IsTtsEnabled(config ResolvedTtsConfig, prefsPath, sessionAuto string) bool {
	return ResolveTtsAutoMode(config, prefsPath, sessionAuto) != AutoOff
}

// SetTtsAutoMode 设置 TTS 自动模式。
// TS 对照: tts.ts L409-416
func SetTtsAutoMode(prefsPath string, mode TtsAutoMode) error {
	return updatePrefs(prefsPath, func(prefs *TtsUserPrefs) {
		if prefs.Tts == nil {
			prefs.Tts = &TtsUserPrefsInner{}
		}
		prefs.Tts.Enabled = nil
		prefs.Tts.Auto = mode
	})
}

// SetTtsEnabled 设置 TTS 启用/禁用。
// TS 对照: tts.ts L418-420
func SetTtsEnabled(prefsPath string, enabled bool) error {
	if enabled {
		return SetTtsAutoMode(prefsPath, AutoAlways)
	}
	return SetTtsAutoMode(prefsPath, AutoOff)
}

// ---------- Provider ----------

// SetTtsProvider 设置偏好 Provider。
// TS 对照: tts.ts L440-444
func SetTtsProvider(prefsPath string, provider TtsProvider) error {
	return updatePrefs(prefsPath, func(prefs *TtsUserPrefs) {
		if prefs.Tts == nil {
			prefs.Tts = &TtsUserPrefsInner{}
		}
		prefs.Tts.Provider = provider
	})
}

// ---------- 最大长度 ----------

// GetTtsMaxLength 获取 TTS 最大文本长度。
// TS 对照: tts.ts L446-449
func GetTtsMaxLength(prefsPath string) int {
	prefs := readPrefs(prefsPath)
	if prefs.Tts != nil && prefs.Tts.MaxLength > 0 {
		return prefs.Tts.MaxLength
	}
	return DefaultTtsMaxLength
}

// SetTtsMaxLength 设置 TTS 最大文本长度。
// TS 对照: tts.ts L451-455
func SetTtsMaxLength(prefsPath string, maxLength int) error {
	return updatePrefs(prefsPath, func(prefs *TtsUserPrefs) {
		if prefs.Tts == nil {
			prefs.Tts = &TtsUserPrefsInner{}
		}
		prefs.Tts.MaxLength = maxLength
	})
}

// ---------- 摘要 ----------

// IsSummarizationEnabled 判断文本摘要是否启用。
// TS 对照: tts.ts L457-460
func IsSummarizationEnabled(prefsPath string) bool {
	prefs := readPrefs(prefsPath)
	if prefs.Tts != nil && prefs.Tts.Summarize != nil {
		return *prefs.Tts.Summarize
	}
	return DefaultTtsSummarize
}

// SetSummarizationEnabled 设置文本摘要启用/禁用。
// TS 对照: tts.ts L462-466
func SetSummarizationEnabled(prefsPath string, enabled bool) error {
	return updatePrefs(prefsPath, func(prefs *TtsUserPrefs) {
		if prefs.Tts == nil {
			prefs.Tts = &TtsUserPrefsInner{}
		}
		prefs.Tts.Summarize = &enabled
	})
}

// ---------- 辅助函数 ----------

// configDir 获取配置目录。
func configDir() string {
	dir := os.Getenv("OPENACOSMI_CONFIG_DIR")
	if dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "openacosmi")
}

// resolveUserPath 解析用户路径。
func resolveUserPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}
