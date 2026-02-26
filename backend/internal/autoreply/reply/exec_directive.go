package reply

import (
	"regexp"
	"strings"
)

// TS 对照: auto-reply/reply/exec/directive.ts

// ExecDirectiveResult /exec 指令提取结果。
type ExecDirectiveResult struct {
	Cleaned             string
	HasDirective        bool
	ExecHost            ExecHost
	ExecSecurity        ExecSecurity
	ExecAsk             ExecAsk
	ExecNode            string
	RawExecHost         string
	RawExecSecurity     string
	RawExecAsk          string
	RawExecNode         string
	HasExecOptions      bool
	InvalidExecHost     bool
	InvalidExecSecurity bool
	InvalidExecAsk      bool
	InvalidExecNode     bool
}

var execDirectiveRe = regexp.MustCompile(`(?i)(?:^|\s)/exec(?:$|\s|:)`)

// parseExecDirectiveArgs 有序扫描 /exec 后的参数。
// TS 对照: exec/directive.ts parseExecDirectiveArgs
// 关键语义：使用 takeToken 有序逐个解析，遇到第一个无法识别的 token 立即停止。
func parseExecDirectiveArgs(raw string) (consumed int, result ExecDirectiveResult) {
	i := 0
	n := len(raw)

	// 跳过开头空白
	for i < n && isSpace(raw[i]) {
		i++
	}
	// 跳过可选冒号
	if i < n && raw[i] == ':' {
		i++
		for i < n && isSpace(raw[i]) {
			i++
		}
	}
	consumed = i

	// takeToken 提取下一个非空白 token，并跳过其后空白。
	// 返回 token 字符串；若无 token 则返回 ""。
	takeToken := func() string {
		if i >= n {
			return ""
		}
		start := i
		for i < n && !isSpace(raw[i]) {
			i++
		}
		if start == i {
			return ""
		}
		tok := raw[start:i]
		// 跳过 token 后的空白
		for i < n && isSpace(raw[i]) {
			i++
		}
		return tok
	}

	// splitToken 将 token 按第一个 = 或 : 拆分为 key/value。
	splitToken := func(tok string) (key, value string, ok bool) {
		eqIdx := strings.IndexByte(tok, '=')
		colIdx := strings.IndexByte(tok, ':')
		idx := -1
		switch {
		case eqIdx == -1 && colIdx == -1:
			return "", "", false
		case eqIdx == -1:
			idx = colIdx
		case colIdx == -1:
			idx = eqIdx
		default:
			if eqIdx < colIdx {
				idx = eqIdx
			} else {
				idx = colIdx
			}
		}
		k := strings.ToLower(strings.TrimSpace(tok[:idx]))
		v := strings.TrimSpace(tok[idx+1:])
		if k == "" {
			return "", "", false
		}
		return k, v, true
	}

tokenLoop:
	for i < n {
		tok := takeToken()
		if tok == "" {
			break
		}
		key, value, ok := splitToken(tok)
		if !ok {
			// 遇到不可识别的 token（无 = 或 :），立即停止（不更新 consumed）。
			break
		}
		switch key {
		case "host":
			result.RawExecHost = value
			switch strings.ToLower(value) {
			case "sandbox":
				result.ExecHost = ExecHostSandbox
			case "gateway":
				result.ExecHost = ExecHostGateway
			case "node":
				result.ExecHost = ExecHostNode
			default:
				result.InvalidExecHost = true
			}
			result.HasExecOptions = true
			consumed = i
		case "security":
			result.RawExecSecurity = value
			switch strings.ToLower(value) {
			case "deny":
				result.ExecSecurity = ExecSecurityDeny
			case "allowlist":
				result.ExecSecurity = ExecSecurityAllowlist
			case "full":
				result.ExecSecurity = ExecSecurityFull
			default:
				result.InvalidExecSecurity = true
			}
			result.HasExecOptions = true
			consumed = i
		case "ask":
			result.RawExecAsk = value
			switch strings.ToLower(value) {
			case "off":
				result.ExecAsk = ExecAskOff
			case "on-miss":
				result.ExecAsk = ExecAskOnMiss
			case "always":
				result.ExecAsk = ExecAskAlways
			default:
				result.InvalidExecAsk = true
			}
			result.HasExecOptions = true
			consumed = i
		case "node":
			result.RawExecNode = value
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				result.InvalidExecNode = true
			} else {
				result.ExecNode = trimmed
			}
			result.HasExecOptions = true
			consumed = i
		default:
			// 不认识的 key=value token，立即跳出外层 for 循环（TS 语义：停止解析）。
			break tokenLoop
		}
	}

	return consumed, result
}

// isSpace 判断字节是否为空白字符。
func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// ExtractExecDirective 从消息体提取 /exec 指令。
// TS 对照: exec/directive.ts extractExecDirective
// 语义修复（D-1）：使用有序 takeToken 扫描，遇到第一个不认识的 token 即停止。
func ExtractExecDirective(body string) ExecDirectiveResult {
	if body == "" {
		return ExecDirectiveResult{Cleaned: "", HasDirective: false}
	}
	loc := execDirectiveRe.FindStringIndex(body)
	if loc == nil {
		return ExecDirectiveResult{Cleaned: strings.TrimSpace(body), HasDirective: false}
	}

	// 找到 /exec 在匹配串中的位置
	matchStr := body[loc[0]:loc[1]]
	execOffset := strings.Index(strings.ToLower(matchStr), "/exec")
	start := loc[0] + execOffset
	argsStart := start + len("/exec")

	consumed, parsed := parseExecDirectiveArgs(body[argsStart:])

	// 拼接清理后的文本：/exec 之前 + /exec 之后已消耗参数之后
	cleanedRaw := body[:start] + " " + body[argsStart+consumed:]
	cleaned := collapseWhitespace(cleanedRaw)

	return ExecDirectiveResult{
		Cleaned:             cleaned,
		HasDirective:        true,
		ExecHost:            parsed.ExecHost,
		ExecSecurity:        parsed.ExecSecurity,
		ExecAsk:             parsed.ExecAsk,
		ExecNode:            parsed.ExecNode,
		RawExecHost:         parsed.RawExecHost,
		RawExecSecurity:     parsed.RawExecSecurity,
		RawExecAsk:          parsed.RawExecAsk,
		RawExecNode:         parsed.RawExecNode,
		HasExecOptions:      parsed.HasExecOptions,
		InvalidExecHost:     parsed.InvalidExecHost,
		InvalidExecSecurity: parsed.InvalidExecSecurity,
		InvalidExecAsk:      parsed.InvalidExecAsk,
		InvalidExecNode:     parsed.InvalidExecNode,
	}
}
