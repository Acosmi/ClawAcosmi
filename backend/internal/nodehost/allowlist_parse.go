package nodehost

// allowlist_parse.go — Shell 命令解析
// 对应 TS: exec-approvals.ts L411-993（tokenize / pipeline / chain / analyze）

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode"
)

// ---------- 命令解析 ----------

// parseFirstToken 提取命令中的第一个 token（可执行文件名）。
func parseFirstToken(command string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return ""
	}
	first := trimmed[0]
	if first == '"' || first == '\'' {
		end := strings.IndexByte(trimmed[1:], first)
		if end > 0 {
			return trimmed[1 : end+1]
		}
		return trimmed[1:]
	}
	idx := strings.IndexFunc(trimmed, unicode.IsSpace)
	if idx > 0 {
		return trimmed[:idx]
	}
	return trimmed
}

// resolveExecutablePath 解析可执行文件路径。
func resolveExecutablePath(rawExec, cwd string, env map[string]string) string {
	expanded := rawExec
	if strings.HasPrefix(expanded, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			expanded = filepath.Join(home, expanded[2:])
		}
	}
	if strings.Contains(expanded, "/") || strings.Contains(expanded, `\`) {
		if filepath.IsAbs(expanded) {
			if isExecFile(expanded) {
				return expanded
			}
			return ""
		}
		base := cwd
		if base == "" {
			base, _ = os.Getwd()
		}
		candidate := filepath.Join(base, expanded)
		if isExecFile(candidate) {
			return candidate
		}
		return ""
	}
	// PATH 查找
	pathEnv := ""
	if v, ok := env["PATH"]; ok {
		pathEnv = v
	} else {
		pathEnv = os.Getenv("PATH")
	}
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, expanded)
		if isExecFile(candidate) {
			return candidate
		}
	}
	return ""
}

func isExecFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS != "windows" {
		// 检查可执行权限
		return info.Mode()&0111 != 0
	}
	return true
}

// ResolveCommandResolution 从 shell 命令解析第一个 token 并查找路径。
func ResolveCommandResolution(command, cwd string, env map[string]string) *CommandResolution {
	raw := parseFirstToken(command)
	if raw == "" {
		return nil
	}
	resolved := resolveExecutablePath(raw, cwd, env)
	name := raw
	if resolved != "" {
		name = filepath.Base(resolved)
	}
	return &CommandResolution{RawExecutable: raw, ResolvedPath: resolved, ExecutableName: name}
}

// ResolveCommandResolutionFromArgv 从 argv 解析命令。
func ResolveCommandResolutionFromArgv(argv []string, cwd string, env map[string]string) *CommandResolution {
	if len(argv) == 0 {
		return nil
	}
	raw := strings.TrimSpace(argv[0])
	if raw == "" {
		return nil
	}
	resolved := resolveExecutablePath(raw, cwd, env)
	name := raw
	if resolved != "" {
		name = filepath.Base(resolved)
	}
	return &CommandResolution{RawExecutable: raw, ResolvedPath: resolved, ExecutableName: name}
}

// ---------- Shell 分词 ----------

// tokenizeShellSegment 对单个管道段进行 Shell 分词。
func tokenizeShellSegment(segment string) ([]string, bool) {
	var tokens []string
	var buf strings.Builder
	inSingle, inDouble, escaped := false, false, false

	pushToken := func() {
		if buf.Len() > 0 {
			tokens = append(tokens, buf.String())
			buf.Reset()
		}
	}

	for i := 0; i < len(segment); i++ {
		ch := segment[i]
		if escaped {
			buf.WriteByte(ch)
			escaped = false
			continue
		}
		if !inSingle && !inDouble && ch == '\\' {
			escaped = true
			continue
		}
		if inSingle {
			if ch == '\'' {
				inSingle = false
			} else {
				buf.WriteByte(ch)
			}
			continue
		}
		if inDouble {
			if ch == '\\' && i+1 < len(segment) {
				next := segment[i+1]
				if next == '\\' || next == '"' || next == '$' || next == '`' {
					buf.WriteByte(next)
					i++
					continue
				}
			}
			if ch == '"' {
				inDouble = false
			} else {
				buf.WriteByte(ch)
			}
			continue
		}
		if ch == '\'' {
			inSingle = true
			continue
		}
		if ch == '"' {
			inDouble = true
			continue
		}
		if unicode.IsSpace(rune(ch)) {
			pushToken()
			continue
		}
		buf.WriteByte(ch)
	}
	if escaped || inSingle || inDouble {
		return nil, false
	}
	pushToken()
	return tokens, true
}

// ---------- 管道拆分 ----------

// splitShellPipeline 按 | 拆分管道（不支持 ||、>&、重定向等）。
func splitShellPipeline(command string) ([]string, bool, string) {
	var parts []string
	var buf strings.Builder
	inSingle, inDouble, escaped := false, false, false
	emptySegment := false

	disallowed := map[byte]bool{'>': true, '<': true, '`': true, '\n': true, '\r': true, '(': true, ')': true}

	pushPart := func() {
		trimmed := strings.TrimSpace(buf.String())
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
		buf.Reset()
	}

	for i := 0; i < len(command); i++ {
		ch := command[i]
		if escaped {
			buf.WriteByte(ch)
			escaped = false
			continue
		}
		if !inSingle && !inDouble && ch == '\\' {
			escaped = true
			buf.WriteByte(ch)
			continue
		}
		if inSingle {
			if ch == '\'' {
				inSingle = false
			}
			buf.WriteByte(ch)
			continue
		}
		if inDouble {
			if ch == '"' {
				inDouble = false
			}
			buf.WriteByte(ch)
			continue
		}
		if ch == '\'' {
			inSingle = true
			buf.WriteByte(ch)
			continue
		}
		if ch == '"' {
			inDouble = true
			buf.WriteByte(ch)
			continue
		}
		if ch == '|' {
			if i+1 < len(command) && command[i+1] == '|' {
				return nil, false, "unsupported: ||"
			}
			if i+1 < len(command) && command[i+1] == '&' {
				return nil, false, "unsupported: |&"
			}
			emptySegment = true
			pushPart()
			continue
		}
		if ch == '&' || ch == ';' {
			return nil, false, "unsupported: " + string(ch)
		}
		if disallowed[ch] {
			return nil, false, "unsupported: " + string(ch)
		}
		if ch == '$' && i+1 < len(command) && command[i+1] == '(' {
			return nil, false, "unsupported: $()"
		}
		emptySegment = false
		buf.WriteByte(ch)
	}
	if escaped || inSingle || inDouble {
		return nil, false, "unterminated quote"
	}
	pushPart()
	if emptySegment || len(parts) == 0 {
		reason := "empty command"
		if len(parts) > 0 {
			reason = "empty pipeline segment"
		}
		return nil, false, reason
	}
	return parts, true, ""
}

// ---------- Chain 拆分 ----------

// splitCommandChain 按 &&/||/; 拆分命令链。
func splitCommandChain(command string) []string {
	var parts []string
	var buf strings.Builder
	inSingle, inDouble, escaped := false, false, false
	foundChain, invalidChain := false, false

	pushPart := func() bool {
		trimmed := strings.TrimSpace(buf.String())
		buf.Reset()
		if trimmed != "" {
			parts = append(parts, trimmed)
			return true
		}
		return false
	}

	for i := 0; i < len(command); i++ {
		ch := command[i]
		if escaped {
			buf.WriteByte(ch)
			escaped = false
			continue
		}
		if !inSingle && !inDouble && ch == '\\' {
			escaped = true
			buf.WriteByte(ch)
			continue
		}
		if inSingle {
			if ch == '\'' {
				inSingle = false
			}
			buf.WriteByte(ch)
			continue
		}
		if inDouble {
			if ch == '"' {
				inDouble = false
			}
			buf.WriteByte(ch)
			continue
		}
		if ch == '\'' {
			inSingle = true
			buf.WriteByte(ch)
			continue
		}
		if ch == '"' {
			inDouble = true
			buf.WriteByte(ch)
			continue
		}
		if ch == '&' && i+1 < len(command) && command[i+1] == '&' {
			if !pushPart() {
				invalidChain = true
			}
			i++
			foundChain = true
			continue
		}
		if ch == '|' && i+1 < len(command) && command[i+1] == '|' {
			if !pushPart() {
				invalidChain = true
			}
			i++
			foundChain = true
			continue
		}
		if ch == ';' {
			if !pushPart() {
				invalidChain = true
			}
			foundChain = true
			continue
		}
		buf.WriteByte(ch)
	}
	pushedFinal := pushPart()
	if !foundChain {
		return nil
	}
	if invalidChain || !pushedFinal || len(parts) == 0 {
		return nil
	}
	return parts
}

// ---------- 命令分析 ----------

// AnalyzeShellCommand 分析 Shell 命令（含管道和 chain）。
// platform 为空时使用当前运行时平台。
func AnalyzeShellCommand(command, cwd string, env map[string]string, platform string) ExecCommandAnalysis {
	if IsWindowsPlatform(platform) {
		return AnalyzeWindowsShellCommand(command, cwd, env)
	}
	chainParts := splitCommandChain(command)
	if chainParts != nil {
		var chains [][]ExecCommandSegment
		var allSegments []ExecCommandSegment
		for _, part := range chainParts {
			pipeSegs, ok, reason := splitShellPipeline(part)
			if !ok {
				return ExecCommandAnalysis{OK: false, Reason: reason}
			}
			segments := parseSegmentsFromParts(pipeSegs, cwd, env)
			if segments == nil {
				return ExecCommandAnalysis{OK: false, Reason: "unable to parse shell segment"}
			}
			chains = append(chains, segments)
			allSegments = append(allSegments, segments...)
		}
		return ExecCommandAnalysis{OK: true, Segments: allSegments, Chains: chains}
	}

	pipeSegs, ok, reason := splitShellPipeline(command)
	if !ok {
		return ExecCommandAnalysis{OK: false, Reason: reason}
	}
	segments := parseSegmentsFromParts(pipeSegs, cwd, env)
	if segments == nil {
		return ExecCommandAnalysis{OK: false, Reason: "unable to parse shell segment"}
	}
	return ExecCommandAnalysis{OK: true, Segments: segments}
}

// AnalyzeArgvCommand 分析 argv 命令。
func AnalyzeArgvCommand(argv []string, cwd string, env map[string]string) ExecCommandAnalysis {
	filtered := make([]string, 0, len(argv))
	for _, a := range argv {
		if strings.TrimSpace(a) != "" {
			filtered = append(filtered, a)
		}
	}
	if len(filtered) == 0 {
		return ExecCommandAnalysis{OK: false, Reason: "empty argv"}
	}
	return ExecCommandAnalysis{
		OK: true,
		Segments: []ExecCommandSegment{{
			Raw:        strings.Join(filtered, " "),
			Argv:       filtered,
			Resolution: ResolveCommandResolutionFromArgv(filtered, cwd, env),
		}},
	}
}

func parseSegmentsFromParts(parts []string, cwd string, env map[string]string) []ExecCommandSegment {
	segments := make([]ExecCommandSegment, 0, len(parts))
	for _, raw := range parts {
		argv, ok := tokenizeShellSegment(raw)
		if !ok || len(argv) == 0 {
			return nil
		}
		segments = append(segments, ExecCommandSegment{
			Raw:        raw,
			Argv:       argv,
			Resolution: ResolveCommandResolutionFromArgv(argv, cwd, env),
		})
	}
	return segments
}

// ---------- Glob 匹配 ----------

// globToRegExp 将 glob 模式转换为正则表达式。
func globToRegExp(pattern string) *regexp.Regexp {
	var buf strings.Builder
	buf.WriteString("(?i)^")
	i := 0
	for i < len(pattern) {
		ch := pattern[i]
		if ch == '*' {
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				buf.WriteString(".*")
				i += 2
				continue
			}
			buf.WriteString("[^/]*")
			i++
			continue
		}
		if ch == '?' {
			buf.WriteByte('.')
			i++
			continue
		}
		buf.WriteString(regexp.QuoteMeta(string(ch)))
		i++
	}
	buf.WriteByte('$')
	re, err := regexp.Compile(buf.String())
	if err != nil {
		return nil
	}
	return re
}
