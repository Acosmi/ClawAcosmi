package daemon

import "strings"

// ParseKeyValueOutput 解析 key=value 或 key: value 格式的命令输出
// 对应 TS: runtime-parse.ts parseKeyValueOutput
func ParseKeyValueOutput(output, separator string) map[string]string {
	entries := make(map[string]string)
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(strings.TrimRight(rawLine, "\r"))
		if line == "" {
			continue
		}
		idx := strings.Index(line, separator)
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(line[:idx]))
		if key == "" {
			continue
		}
		value := strings.TrimSpace(line[idx+len(separator):])
		entries[key] = value
	}
	return entries
}
