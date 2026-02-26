package autoreply

import (
	"fmt"
	"strings"
)

// TS 对照: auto-reply/commands-args.ts

// NormalizeArgValue 规范化参数值为字符串。
// TS 对照: commands-args.ts L5-23
func NormalizeArgValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case int, int64, float64, bool:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	default:
		return ""
	}
}

// FormatConfigArgs 格式化 config 命令参数。
// TS 对照: commands-args.ts L25-48
func FormatConfigArgs(values CommandArgValues) string {
	action := strings.ToLower(NormalizeArgValue(values["action"]))
	path := NormalizeArgValue(values["path"])
	value := NormalizeArgValue(values["value"])
	if action == "" {
		return ""
	}
	if action == "show" || action == "get" {
		if path != "" {
			return action + " " + path
		}
		return action
	}
	if action == "unset" {
		if path != "" {
			return action + " " + path
		}
		return action
	}
	if action == "set" {
		if path == "" {
			return action
		}
		if value == "" {
			return action + " " + path
		}
		return action + " " + path + "=" + value
	}
	return action
}

// FormatDebugArgs 格式化 debug 命令参数。
// TS 对照: commands-args.ts L50-73
func FormatDebugArgs(values CommandArgValues) string {
	action := strings.ToLower(NormalizeArgValue(values["action"]))
	path := NormalizeArgValue(values["path"])
	value := NormalizeArgValue(values["value"])
	if action == "" {
		return ""
	}
	if action == "show" || action == "reset" {
		return action
	}
	if action == "unset" {
		if path != "" {
			return action + " " + path
		}
		return action
	}
	if action == "set" {
		if path == "" {
			return action
		}
		if value == "" {
			return action + " " + path
		}
		return action + " " + path + "=" + value
	}
	return action
}

// FormatQueueArgs 格式化 queue 命令参数。
// TS 对照: commands-args.ts L75-94
func FormatQueueArgs(values CommandArgValues) string {
	mode := NormalizeArgValue(values["mode"])
	debounce := NormalizeArgValue(values["debounce"])
	cap := NormalizeArgValue(values["cap"])
	drop := NormalizeArgValue(values["drop"])
	var parts []string
	if mode != "" {
		parts = append(parts, mode)
	}
	if debounce != "" {
		parts = append(parts, "debounce:"+debounce)
	}
	if cap != "" {
		parts = append(parts, "cap:"+cap)
	}
	if drop != "" {
		parts = append(parts, "drop:"+drop)
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	return ""
}

// CommandArgFormatters 预注册的参数格式化器映射。
// TS 对照: commands-args.ts L96-100
var CommandArgFormatters = map[string]CommandArgsFormatter{
	"config": FormatConfigArgs,
	"debug":  FormatDebugArgs,
	"queue":  FormatQueueArgs,
}
