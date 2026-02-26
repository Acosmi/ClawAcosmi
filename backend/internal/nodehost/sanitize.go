package nodehost

// sanitize.go — 环境变量消毒 + 命令格式化 + 输出截断
// 对应 TS: runner.ts L165-248 (sanitizeEnv) + L350-371 (format/truncate)

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// blockedEnvKeys 禁止从客户端覆盖的环境变量。
var blockedEnvKeys = map[string]struct{}{
	"NODE_OPTIONS": {},
	"PYTHONHOME":   {},
	"PYTHONPATH":   {},
	"PERL5LIB":     {},
	"PERL5OPT":     {},
	"RUBYOPT":      {},
}

// blockedEnvPrefixes 禁止从客户端覆盖的环境变量前缀。
var blockedEnvPrefixes = []string{"DYLD_", "LD_"}

// SanitizeEnv 对客户端提供的环境变量进行消毒。
// 过滤危险变量，限制 PATH 仅允许追加。
func SanitizeEnv(overrides map[string]string) map[string]string {
	if len(overrides) == 0 {
		return nil
	}

	merged := make(map[string]string)
	for _, kv := range os.Environ() {
		if idx := strings.IndexByte(kv, '='); idx >= 0 {
			merged[kv[:idx]] = kv[idx+1:]
		}
	}

	basePath := os.Getenv("PATH")
	if basePath == "" {
		basePath = DefaultNodePath
	}

	for rawKey, value := range overrides {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			continue
		}
		upper := strings.ToUpper(key)

		// PATH 特殊处理：仅允许在原始 PATH 后追加
		if upper == "PATH" {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			if basePath == "" || trimmed == basePath {
				merged[key] = trimmed
				continue
			}
			suffix := string(filepath.ListSeparator) + basePath
			if strings.HasSuffix(trimmed, suffix) {
				merged[key] = trimmed
			}
			continue
		}

		// 黑名单过滤
		if _, blocked := blockedEnvKeys[upper]; blocked {
			continue
		}
		blocked := false
		for _, prefix := range blockedEnvPrefixes {
			if strings.HasPrefix(upper, prefix) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}

		merged[key] = value
	}
	return merged
}

// FormatCommand 将 argv 格式化为可读命令字符串。
func FormatCommand(argv []string) string {
	parts := make([]string, 0, len(argv))
	for _, arg := range argv {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" {
			parts = append(parts, `""`)
			continue
		}
		if strings.ContainsAny(trimmed, " \t\"") {
			parts = append(parts, fmt.Sprintf(`"%s"`, strings.ReplaceAll(trimmed, `"`, `\"`)))
		} else {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, " ")
}

// TruncateOutput 截断输出到指定字节数。
func TruncateOutput(raw string, maxChars int) (text string, truncated bool) {
	if len(raw) <= maxChars {
		return raw, false
	}
	return "... (truncated) " + raw[len(raw)-maxChars:], true
}

// BuildExecEventPayload 构建执行事件载荷，截断 output。
func BuildExecEventPayload(p *ExecEventPayload) *ExecEventPayload {
	if p.Output == "" {
		return p
	}
	trimmed := strings.TrimSpace(p.Output)
	if trimmed == "" {
		return p
	}
	text, _ := TruncateOutput(trimmed, OutputEventTail)
	cp := *p
	cp.Output = text
	return &cp
}
