package daemon

import (
	"os"
	"regexp"
	"strings"
)

// gatewayLogErrorPatterns 网关日志错误模式
var gatewayLogErrorPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)refusing to bind gateway`),
	regexp.MustCompile(`(?i)gateway auth mode`),
	regexp.MustCompile(`(?i)gateway start blocked`),
	regexp.MustCompile(`(?i)failed to bind gateway socket`),
	regexp.MustCompile(`(?i)tailscale .* requires`),
}

// ReadLastGatewayErrorLine 从网关日志中读取最后一条错误行
// 对应 TS: diagnostics.ts readLastGatewayErrorLine
func ReadLastGatewayErrorLine(env map[string]string) string {
	stdoutPath, stderrPath := ResolveGatewayLogPaths(env)

	stderrRaw, _ := os.ReadFile(stderrPath)
	stdoutRaw, _ := os.ReadFile(stdoutPath)

	var allLines []string
	for _, raw := range [][]byte{stderrRaw, stdoutRaw} {
		for _, line := range strings.Split(string(raw), "\n") {
			allLines = append(allLines, strings.TrimSpace(line))
		}
	}

	// 从后往前搜索匹配的错误行
	for i := len(allLines) - 1; i >= 0; i-- {
		line := allLines[i]
		if line == "" {
			continue
		}
		for _, pattern := range gatewayLogErrorPatterns {
			if pattern.MatchString(line) {
				return line
			}
		}
	}

	// 没有匹配的错误模式，返回最后一行
	return readLastLine(stderrPath)
}

// readLastLine 读取文件最后一个非空行
func readLastLine(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
}
