// bash/pty_keys.go — PTY 键映射 + 编码。
// TS 参考：src/agents/pty-keys.ts (294L)
//
// 提供 key name → ANSI escape sequence 映射，
// 支持修饰符（Ctrl/Alt/Shift）和 xterm modifier CSI 编码。
package bash

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ---------- ANSI 常量 ----------

const (
	ESC       = "\x1b"
	CR        = "\r"
	TAB       = "\t"
	BACKSPACE = "\x7f"

	BracketedPasteStart = ESC + "[200~"
	BracketedPasteEnd   = ESC + "[201~"
)

// ---------- Key Map ----------

// namedKeyMap 名称 → ANSI 序列映射。
// TS 参考: pty-keys.ts L19-74
var namedKeyMap = map[string]string{
	"enter":     CR,
	"return":    CR,
	"tab":       TAB,
	"escape":    ESC,
	"esc":       ESC,
	"space":     " ",
	"bspace":    BACKSPACE,
	"backspace": BACKSPACE,
	"up":        ESC + "[A",
	"down":      ESC + "[B",
	"right":     ESC + "[C",
	"left":      ESC + "[D",
	"home":      ESC + "[1~",
	"end":       ESC + "[4~",
	"pageup":    ESC + "[5~",
	"pgup":      ESC + "[5~",
	"ppage":     ESC + "[5~",
	"pagedown":  ESC + "[6~",
	"pgdn":      ESC + "[6~",
	"npage":     ESC + "[6~",
	"insert":    ESC + "[2~",
	"ic":        ESC + "[2~",
	"delete":    ESC + "[3~",
	"del":       ESC + "[3~",
	"dc":        ESC + "[3~",
	"btab":      ESC + "[Z",
	"f1":        ESC + "OP",
	"f2":        ESC + "OQ",
	"f3":        ESC + "OR",
	"f4":        ESC + "OS",
	"f5":        ESC + "[15~",
	"f6":        ESC + "[17~",
	"f7":        ESC + "[18~",
	"f8":        ESC + "[19~",
	"f9":        ESC + "[20~",
	"f10":       ESC + "[21~",
	"f11":       ESC + "[23~",
	"f12":       ESC + "[24~",
	"kp/":       ESC + "Oo",
	"kp*":       ESC + "Oj",
	"kp-":       ESC + "Om",
	"kp+":       ESC + "Ok",
	"kp7":       ESC + "Ow",
	"kp8":       ESC + "Ox",
	"kp9":       ESC + "Oy",
	"kp4":       ESC + "Ot",
	"kp5":       ESC + "Ou",
	"kp6":       ESC + "Ov",
	"kp1":       ESC + "Oq",
	"kp2":       ESC + "Or",
	"kp3":       ESC + "Os",
	"kp0":       ESC + "Op",
	"kp.":       ESC + "On",
	"kpenter":   ESC + "OM",
}

// modifiableNamedKeys 可以使用 xterm CSI modifier 的键。
var modifiableNamedKeys = map[string]bool{
	"up": true, "down": true, "left": true, "right": true,
	"home": true, "end": true,
	"pageup": true, "pgup": true, "ppage": true,
	"pagedown": true, "pgdn": true, "npage": true,
	"insert": true, "ic": true,
	"delete": true, "del": true, "dc": true,
}

// ---------- 请求/结果类型 ----------

// KeyEncodingRequest 键序列编码请求。
// TS 参考: pty-keys.ts L96-100
type KeyEncodingRequest struct {
	Keys    []string `json:"keys,omitempty"`
	Hex     []string `json:"hex,omitempty"`
	Literal string   `json:"literal,omitempty"`
}

// KeyEncodingResult 键序列编码结果。
type KeyEncodingResult struct {
	Data     string   `json:"data"`
	Warnings []string `json:"warnings,omitempty"`
}

// ---------- 公开函数 ----------

// EncodeKeySequence 编码键序列。
// TS 参考: pty-keys.ts L107-133
func EncodeKeySequence(request KeyEncodingRequest) KeyEncodingResult {
	var warnings []string
	var data strings.Builder

	if request.Literal != "" {
		data.WriteString(request.Literal)
	}

	for _, raw := range request.Hex {
		b, err := parseHexByte(raw)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Invalid hex byte: %s", raw))
			continue
		}
		data.WriteByte(b)
	}

	for _, token := range request.Keys {
		data.WriteString(encodeKeyToken(token, &warnings))
	}

	return KeyEncodingResult{
		Data:     data.String(),
		Warnings: warnings,
	}
}

// EncodePaste 包裹文本为 bracketed paste 序列。
// TS 参考: pty-keys.ts L135-140
func EncodePaste(text string, bracketed bool) string {
	if !bracketed {
		return text
	}
	return BracketedPasteStart + text + BracketedPasteEnd
}

// ---------- 内部函数 ----------

// modifiers 修饰符状态。
type modifiers struct {
	ctrl  bool
	alt   bool
	shift bool
}

func encodeKeyToken(raw string, warnings *[]string) string {
	token := strings.TrimSpace(raw)
	if token == "" {
		return ""
	}

	// ^X 快捷语法
	if len(token) == 2 && token[0] == '^' {
		ctrl := toCtrlChar(rune(token[1]))
		if ctrl != "" {
			return ctrl
		}
	}

	parsed := parseModifiers(token)
	baseLower := strings.ToLower(parsed.base)

	// Shift+Tab 特殊处理
	if baseLower == "tab" && parsed.mods.shift {
		return ESC + "[Z"
	}

	baseSeq, found := namedKeyMap[baseLower]
	if found {
		seq := baseSeq
		if modifiableNamedKeys[baseLower] && hasAnyModifier(parsed.mods) {
			mod := xtermModifier(parsed.mods)
			if mod > 1 {
				modified := applyXtermModifier(seq, mod)
				if modified != "" {
					return modified
				}
			}
		}
		if parsed.mods.alt {
			return ESC + seq
		}
		return seq
	}

	// 单字符
	runes := []rune(parsed.base)
	if len(runes) == 1 {
		return applyCharModifiers(runes[0], parsed.mods)
	}

	if parsed.hasModifiers {
		*warnings = append(*warnings, fmt.Sprintf("Unknown key %q for modifiers; sending literal.", parsed.base))
	}
	return parsed.base
}

type parsedToken struct {
	mods         modifiers
	base         string
	hasModifiers bool
}

func parseModifiers(token string) parsedToken {
	mods := modifiers{}
	rest := token
	sawModifiers := false

	for len(rest) > 2 && rest[1] == '-' {
		mod := unicode.ToLower(rune(rest[0]))
		switch mod {
		case 'c':
			mods.ctrl = true
		case 'm':
			mods.alt = true
		case 's':
			mods.shift = true
		default:
			return parsedToken{mods: mods, base: rest, hasModifiers: sawModifiers}
		}
		sawModifiers = true
		rest = rest[2:]
	}

	return parsedToken{mods: mods, base: rest, hasModifiers: sawModifiers}
}

func applyCharModifiers(char rune, mods modifiers) string {
	value := string(char)
	if mods.shift && char >= 'a' && char <= 'z' {
		value = strings.ToUpper(value)
	}
	if mods.ctrl {
		ctrl := toCtrlChar(rune(value[0]))
		if ctrl != "" {
			value = ctrl
		}
	}
	if mods.alt {
		value = ESC + value
	}
	return value
}

func toCtrlChar(char rune) string {
	if char == '?' {
		return "\x7f"
	}
	code := unicode.ToUpper(char)
	if code >= 64 && code <= 95 {
		return string(rune(code & 0x1f))
	}
	return ""
}

func xtermModifier(mods modifiers) int {
	mod := 1
	if mods.shift {
		mod += 1
	}
	if mods.alt {
		mod += 2
	}
	if mods.ctrl {
		mod += 4
	}
	return mod
}

// csiNumberRe 匹配 CSI number 序列: ESC[<n><letter>
var csiNumberRe = regexp.MustCompile(`^\x1b\[(\d+)([~A-Z])$`)

// csiArrowRe 匹配 CSI cursor 序列: ESC[<letter>
var csiArrowRe = regexp.MustCompile(`^\x1b\[([A-DHWF])$`)

func applyXtermModifier(sequence string, modifier int) string {
	if m := csiNumberRe.FindStringSubmatch(sequence); m != nil {
		return fmt.Sprintf("%s[%s;%d%s", ESC, m[1], modifier, m[2])
	}
	if m := csiArrowRe.FindStringSubmatch(sequence); m != nil {
		return fmt.Sprintf("%s[1;%d%s", ESC, modifier, m[1])
	}
	return ""
}

func hasAnyModifier(mods modifiers) bool {
	return mods.ctrl || mods.alt || mods.shift
}

func parseHexByte(raw string) (byte, error) {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	normalized := strings.TrimPrefix(trimmed, "0x")
	matched, _ := regexp.MatchString(`^[0-9a-f]{1,2}$`, normalized)
	if !matched {
		return 0, fmt.Errorf("invalid hex: %s", raw)
	}
	value, err := strconv.ParseUint(normalized, 16, 8)
	if err != nil {
		return 0, err
	}
	return byte(value), nil
}
