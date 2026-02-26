package config

// Telegram 自定义命令解析 — 对应 src/config/telegram-custom-commands.ts (96 行)
//
// 验证和规范化 Telegram 自定义命令配置。
// 支持命令名验证（a-z0-9_，最多 32 字符）、保留命令冲突检测、重复检测。

import (
	"fmt"
	"regexp"
	"strings"
)

// TelegramCommandNamePattern 合法的 Telegram 命令名模式
// 对应 TS: TELEGRAM_COMMAND_NAME_PATTERN = /^[a-z0-9_]{1,32}$/
var TelegramCommandNamePattern = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)

// TelegramCustomCommandInput 自定义命令输入
type TelegramCustomCommandInput struct {
	Command     string `json:"command,omitempty"`
	Description string `json:"description,omitempty"`
}

// TelegramCustomCommandIssue 命令解析问题
type TelegramCustomCommandIssue struct {
	Index   int    `json:"index"`
	Field   string `json:"field"` // "command" | "description"
	Message string `json:"message"`
}

// TelegramResolvedCommand 解析后的命令
type TelegramResolvedCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// NormalizeTelegramCommandName 规范化 Telegram 命令名。
// 去除前导 /、转小写、去空白。
// 对应 TS: normalizeTelegramCommandName(value)
func NormalizeTelegramCommandName(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimPrefix(trimmed, "/")
	return strings.ToLower(strings.TrimSpace(trimmed))
}

// NormalizeTelegramCommandDescription 规范化命令描述。
// 对应 TS: normalizeTelegramCommandDescription(value)
func NormalizeTelegramCommandDescription(value string) string {
	return strings.TrimSpace(value)
}

// ResolveTelegramCustomCommandsOptions 解析选项
type ResolveTelegramCustomCommandsOptions struct {
	Commands         []TelegramCustomCommandInput
	ReservedCommands map[string]bool
	CheckReserved    bool // 默认 true
	CheckDuplicates  bool // 默认 true
}

// ResolveTelegramCustomCommands 验证和规范化 Telegram 自定义命令。
// 对应 TS: resolveTelegramCustomCommands(params)
func ResolveTelegramCustomCommands(opts ResolveTelegramCustomCommandsOptions) ([]TelegramResolvedCommand, []TelegramCustomCommandIssue) {
	checkReserved := opts.CheckReserved
	checkDuplicates := opts.CheckDuplicates

	seen := make(map[string]bool)
	var resolved []TelegramResolvedCommand
	var issues []TelegramCustomCommandIssue

	for i, entry := range opts.Commands {
		normalized := NormalizeTelegramCommandName(entry.Command)
		if normalized == "" {
			issues = append(issues, TelegramCustomCommandIssue{
				Index:   i,
				Field:   "command",
				Message: "Telegram custom command is missing a command name.",
			})
			continue
		}
		if !TelegramCommandNamePattern.MatchString(normalized) {
			issues = append(issues, TelegramCustomCommandIssue{
				Index:   i,
				Field:   "command",
				Message: fmt.Sprintf("Telegram custom command \"/%s\" is invalid (use a-z, 0-9, underscore; max 32 chars).", normalized),
			})
			continue
		}
		if checkReserved && opts.ReservedCommands[normalized] {
			issues = append(issues, TelegramCustomCommandIssue{
				Index:   i,
				Field:   "command",
				Message: fmt.Sprintf("Telegram custom command \"/%s\" conflicts with a native command.", normalized),
			})
			continue
		}
		if checkDuplicates && seen[normalized] {
			issues = append(issues, TelegramCustomCommandIssue{
				Index:   i,
				Field:   "command",
				Message: fmt.Sprintf("Telegram custom command \"/%s\" is duplicated.", normalized),
			})
			continue
		}
		description := NormalizeTelegramCommandDescription(entry.Description)
		if description == "" {
			issues = append(issues, TelegramCustomCommandIssue{
				Index:   i,
				Field:   "description",
				Message: fmt.Sprintf("Telegram custom command \"/%s\" is missing a description.", normalized),
			})
			continue
		}
		if checkDuplicates {
			seen[normalized] = true
		}
		resolved = append(resolved, TelegramResolvedCommand{
			Command:     normalized,
			Description: description,
		})
	}

	return resolved, issues
}
