package slack

import (
	"regexp"
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// Slack 斜杠命令 — 继承自 src/slack/monitor/commands.ts (24L)

// NormalizeSlackSlashCommandName 规范化斜杠命令名（去除前导 /）。
func NormalizeSlackSlashCommandName(raw string) string {
	return strings.TrimLeft(raw, "/")
}

// ResolvedSlackSlashCommand 解析后的斜杠命令配置
type ResolvedSlackSlashCommand struct {
	Enabled       bool
	Name          string
	SessionPrefix string
	Ephemeral     bool
}

// ResolveSlackSlashCommandConfig 解析斜杠命令配置。
func ResolveSlackSlashCommandConfig(raw *types.SlackSlashCommandConfig) ResolvedSlackSlashCommand {
	if raw == nil {
		return ResolvedSlackSlashCommand{
			Enabled:       false,
			Name:          "openacosmi",
			SessionPrefix: "slack:slash",
			Ephemeral:     true,
		}
	}

	name := NormalizeSlackSlashCommandName(strings.TrimSpace(raw.Name))
	if name == "" {
		name = "openacosmi"
	}

	sessionPrefix := strings.TrimSpace(raw.SessionPrefix)
	if sessionPrefix == "" {
		sessionPrefix = "slack:slash"
	}

	ephemeral := true
	if raw.Ephemeral != nil {
		ephemeral = *raw.Ephemeral
	}

	return ResolvedSlackSlashCommand{
		Enabled:       raw.Enabled != nil && *raw.Enabled,
		Name:          name,
		SessionPrefix: sessionPrefix,
		Ephemeral:     ephemeral,
	}
}

// BuildSlackSlashCommandMatcher 构建斜杠命令正则匹配器。
func BuildSlackSlashCommandMatcher(name string) *regexp.Regexp {
	normalized := NormalizeSlackSlashCommandName(name)
	escaped := regexp.QuoteMeta(normalized)
	return regexp.MustCompile(`^/?` + escaped + `$`)
}
