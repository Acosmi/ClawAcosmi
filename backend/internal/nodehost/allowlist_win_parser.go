package nodehost

// allowlist_windows.go — Windows Shell 命令分析
// 对应 TS: exec-approvals.ts L783-861
//
// Windows 平台使用不同的分词规则：
//   - 仅支持双引号（不支持单引号）
//   - 不支持管道、重定向、环境变量展开等 shell 特性
//   - 命令链（&&/||/;）不支持

import "strings"

// windowsUnsupportedTokens Windows 不支持的 shell token 字符集。
// 对应 TS: WINDOWS_UNSUPPORTED_TOKENS
var windowsUnsupportedTokens = map[byte]bool{
	'&':  true,
	'|':  true,
	'<':  true,
	'>':  true,
	'^':  true,
	'(':  true,
	')':  true,
	'%':  true,
	'!':  true,
	'\n': true,
	'\r': true,
}

// findWindowsUnsupportedToken 扫描命令字符串，返回第一个不支持的 token。
// 对应 TS: findWindowsUnsupportedToken (L783-793)
func findWindowsUnsupportedToken(command string) string {
	for i := 0; i < len(command); i++ {
		ch := command[i]
		if windowsUnsupportedTokens[ch] {
			if ch == '\n' || ch == '\r' {
				return "newline"
			}
			return string(ch)
		}
	}
	return ""
}

// tokenizeWindowsSegment 对 Windows 命令进行分词。
// Windows 分词规则：仅双引号，空白分隔，不支持转义。
// 对应 TS: tokenizeWindowsSegment (L795-825)
func tokenizeWindowsSegment(segment string) []string {
	var tokens []string
	var buf strings.Builder
	inDouble := false

	pushToken := func() {
		if buf.Len() > 0 {
			tokens = append(tokens, buf.String())
			buf.Reset()
		}
	}

	for i := 0; i < len(segment); i++ {
		ch := segment[i]
		if ch == '"' {
			inDouble = !inDouble
			continue
		}
		if !inDouble && (ch == ' ' || ch == '\t') {
			pushToken()
			continue
		}
		buf.WriteByte(ch)
	}

	if inDouble {
		// 未闭合的双引号 → 解析失败
		return nil
	}
	pushToken()
	return tokens
}

// AnalyzeWindowsShellCommand 分析 Windows Shell 命令。
// 对应 TS: analyzeWindowsShellCommand (L827-854)
func AnalyzeWindowsShellCommand(command, cwd string, env map[string]string) ExecCommandAnalysis {
	if tok := findWindowsUnsupportedToken(command); tok != "" {
		return ExecCommandAnalysis{
			OK:     false,
			Reason: "unsupported windows shell token: " + tok,
		}
	}
	argv := tokenizeWindowsSegment(command)
	if len(argv) == 0 {
		return ExecCommandAnalysis{OK: false, Reason: "unable to parse windows command"}
	}
	return ExecCommandAnalysis{
		OK: true,
		Segments: []ExecCommandSegment{{
			Raw:        command,
			Argv:       argv,
			Resolution: ResolveCommandResolutionFromArgv(argv, cwd, env),
		}},
	}
}

// IsWindowsPlatform 判断平台字符串是否为 Windows。
// 对应 TS: isWindowsPlatform (L856-861)
func IsWindowsPlatform(platform string) bool {
	normalized := strings.ToLower(strings.TrimSpace(platform))
	return strings.HasPrefix(normalized, "win")
}
