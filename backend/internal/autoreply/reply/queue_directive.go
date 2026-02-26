package reply

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// TS 对照: auto-reply/reply/queue/directive.ts (197L)
// + cli/parse-duration.ts (40L) + queue/normalize.ts (45L)
//
// 本文件使用 tokenizer 架构，逐 token 扫描 /queue 指令参数。
// 对齐 TS 的三个关键语义：
// 1. 遇到无法识别的 token 立即停止解析
// 2. consumed 偏移精确切割 cleaned 文本
// 3. parseDurationMs 支持 h/d 单位和复合时长

// QueueDirectiveResult /queue 指令提取结果。
type QueueDirectiveResult struct {
	Cleaned         string
	HasDirective    bool
	QueueMode       QueueMode
	QueueReset      bool
	RawQueueMode    string
	DebounceMs      *int
	Cap             *int
	DropPolicy      QueueDropPolicy
	RawDebounce     string
	RawCap          string
	RawDrop         string
	HasQueueOptions bool
}

// QueueSettings 队列设置。
type QueueSettings struct {
	Mode       QueueMode
	DebounceMs *int
	Cap        *int
	DropPolicy QueueDropPolicy
}

// queueDirectiveParseResult 内部解析中间结果。
type queueDirectiveParseResult struct {
	consumed    int
	queueMode   QueueMode
	queueReset  bool
	rawMode     string
	debounceMs  *int
	cap         *int
	dropPolicy  QueueDropPolicy
	rawDebounce string
	rawCap      string
	rawDrop     string
	hasOptions  bool
}

// parseQueueDirectiveArgs 使用 tokenizer 解析 /queue 后的参数。
// TS 对照: parseQueueDirectiveArgs(raw) in directive.ts L35-141
func parseQueueDirectiveArgs(raw string) queueDirectiveParseResult {
	i := 0
	length := len(raw)

	// 跳过前导空白
	for i < length && isWhitespace(raw[i]) {
		i++
	}

	// 跳过可选的 ':' 分隔符
	if i < length && raw[i] == ':' {
		i++
		for i < length && isWhitespace(raw[i]) {
			i++
		}
	}

	consumed := i
	result := queueDirectiveParseResult{}

	// takeToken 提取下一个非空白 token。
	takeToken := func() (string, bool) {
		if i >= length {
			return "", false
		}
		start := i
		for i < length && !isWhitespace(raw[i]) {
			i++
		}
		if start == i {
			return "", false
		}
		token := raw[start:i]
		// 跳过 token 后的空白
		for i < length && isWhitespace(raw[i]) {
			i++
		}
		return token, true
	}

	for i < length {
		token, ok := takeToken()
		if !ok {
			break
		}
		lowered := strings.ToLower(strings.TrimSpace(token))

		// reset/default/clear → 立即 break
		if lowered == "default" || lowered == "reset" || lowered == "clear" {
			result.queueReset = true
			consumed = i
			break
		}

		// debounce:value 或 debounce=value
		if strings.HasPrefix(lowered, "debounce:") || strings.HasPrefix(lowered, "debounce=") {
			parts := splitOnFirstSep(token)
			result.rawDebounce = parts
			ms := parseQueueDebounce(parts)
			if ms != nil {
				result.debounceMs = ms
			}
			result.hasOptions = true
			consumed = i
			continue
		}

		// cap:value 或 cap=value
		if strings.HasPrefix(lowered, "cap:") || strings.HasPrefix(lowered, "cap=") {
			parts := splitOnFirstSep(token)
			result.rawCap = parts
			cap := parseQueueCap(parts)
			if cap != nil {
				result.cap = cap
			}
			result.hasOptions = true
			consumed = i
			continue
		}

		// drop:value 或 drop=value
		if strings.HasPrefix(lowered, "drop:") || strings.HasPrefix(lowered, "drop=") {
			parts := splitOnFirstSep(token)
			result.rawDrop = parts
			dp := NormalizeQueueDropPolicy(parts)
			if dp != "" {
				result.dropPolicy = dp
			}
			result.hasOptions = true
			consumed = i
			continue
		}

		// 尝试作为 queue mode
		mode := NormalizeQueueMode(token)
		if mode != "" {
			result.queueMode = mode
			result.rawMode = token
			consumed = i
			continue
		}

		// 无法识别的 token → 立即停止解析（关键语义对齐 TS）
		break
	}

	result.consumed = consumed
	return result
}

// ExtractQueueDirective 从消息体提取 /queue 指令。
// TS 对照: extractQueueDirective(body) in directive.ts L144-196
func ExtractQueueDirective(body string) QueueDirectiveResult {
	if body == "" {
		return QueueDirectiveResult{Cleaned: "", HasDirective: false}
	}

	// 查找 /queue 位置
	idx := findQueueDirective(body)
	if idx < 0 {
		return QueueDirectiveResult{Cleaned: strings.TrimSpace(body), HasDirective: false}
	}

	argsStart := idx + len("/queue")
	args := body[argsStart:]
	parsed := parseQueueDirectiveArgs(args)

	// 精确切割 cleaned 文本：保留 /queue 之前的部分 + consumed 之后的部分
	before := body[:idx]
	after := ""
	if argsStart+parsed.consumed < len(body) {
		after = body[argsStart+parsed.consumed:]
	}
	cleanedRaw := before + " " + after
	cleaned := collapseWhitespace(cleanedRaw)

	return QueueDirectiveResult{
		Cleaned:         cleaned,
		HasDirective:    true,
		QueueMode:       parsed.queueMode,
		QueueReset:      parsed.queueReset,
		RawQueueMode:    parsed.rawMode,
		DebounceMs:      parsed.debounceMs,
		Cap:             parsed.cap,
		DropPolicy:      parsed.dropPolicy,
		RawDebounce:     parsed.rawDebounce,
		RawCap:          parsed.rawCap,
		RawDrop:         parsed.rawDrop,
		HasQueueOptions: parsed.hasOptions,
	}
}

// findQueueDirective 在 body 中查找 /queue 的位置。
// TS: /(?:^|\s)\/queue(?=$|\s|:)/i
func findQueueDirective(body string) int {
	lower := strings.ToLower(body)
	searchFrom := 0
	for {
		idx := strings.Index(lower[searchFrom:], "/queue")
		if idx < 0 {
			return -1
		}
		absIdx := searchFrom + idx

		// 检查前置条件：必须在行首或前面是空白
		if absIdx > 0 && !isWhitespace(body[absIdx-1]) {
			searchFrom = absIdx + 1
			continue
		}

		// 检查后置条件：必须在行尾、空白或 ':'
		afterIdx := absIdx + len("/queue")
		if afterIdx < len(body) {
			ch := body[afterIdx]
			if !isWhitespace(ch) && ch != ':' {
				searchFrom = absIdx + 1
				continue
			}
		}

		return absIdx
	}
}

// splitOnFirstSep 在第一个 ':' 或 '=' 处分割，返回值部分。
func splitOnFirstSep(token string) string {
	for i, ch := range token {
		if ch == ':' || ch == '=' {
			return token[i+1:]
		}
	}
	return ""
}

// parseQueueDebounce 解析 debounce 值。
// TS 对照: parseQueueDebounce(raw) in directive.ts L5-18
func parseQueueDebounce(raw string) *int {
	if raw == "" {
		return nil
	}
	ms, err := parseDurationMs(strings.TrimSpace(raw), "ms")
	if err != nil || ms < 0 {
		return nil
	}
	rounded := int(math.Round(ms))
	return &rounded
}

// parseQueueCap 解析 cap 值。
// TS 对照: parseQueueCap(raw) in directive.ts L20-33
func parseQueueCap(raw string) *int {
	if raw == "" {
		return nil
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || !isFinite(n) {
		return nil
	}
	cap := int(math.Floor(n))
	if cap < 1 {
		return nil
	}
	return &cap
}

// parseDurationMs 解析时间值为毫秒。
// 支持单位: ms, s, m, h, d 和复合时长 (如 1h30m)。
// TS 对照: parseDurationMs(raw, opts) in cli/parse-duration.ts
func parseDurationMs(raw string, defaultUnit string) (float64, error) {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return 0, strconv.ErrSyntax
	}

	// 先尝试 Go 标准 time.ParseDuration（支持复合时长如 1h30m）
	if d, err := time.ParseDuration(trimmed); err == nil {
		return float64(d.Milliseconds()), nil
	}

	// 尝试单位解析: 数字 + 可选单位
	// 匹配模式: 数字(ms|s|m|h|d)?
	numEnd := 0
	for numEnd < len(trimmed) {
		ch := trimmed[numEnd]
		if (ch >= '0' && ch <= '9') || ch == '.' {
			numEnd++
		} else {
			break
		}
	}
	if numEnd == 0 {
		return 0, strconv.ErrSyntax
	}

	numStr := trimmed[:numEnd]
	unitStr := trimmed[numEnd:]

	value, err := strconv.ParseFloat(numStr, 64)
	if err != nil || !isFinite(value) || value < 0 {
		return 0, strconv.ErrSyntax
	}

	if unitStr == "" {
		unitStr = defaultUnit
	}

	var multiplier float64
	switch unitStr {
	case "ms":
		multiplier = 1
	case "s":
		multiplier = 1000
	case "m":
		multiplier = 60_000
	case "h":
		multiplier = 3_600_000
	case "d":
		multiplier = 86_400_000
	default:
		return 0, strconv.ErrSyntax
	}

	ms := value * multiplier
	if !isFinite(ms) {
		return 0, strconv.ErrSyntax
	}
	return ms, nil
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func isFinite(f float64) bool {
	return !math.IsInf(f, 0) && !math.IsNaN(f)
}
