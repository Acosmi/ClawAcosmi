package config

// 配置 Schema 核心 — 对应 src/config/schema.ts + zod-schema.ts
// 提供配置验证入口、UI 提示、以及 schema 元数据

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// ConfigSchemaResponse schema 查询响应（供 API 使用）
type ConfigSchemaResponse struct {
	Schema      interface{}       `json:"schema"`
	UIHints     map[string]UIHint `json:"uiHints"`
	Version     string            `json:"version"`
	GeneratedAt string            `json:"generatedAt"`
}

// UIHint 配置字段的 UI 提示
type UIHint struct {
	Label        string      `json:"label,omitempty"`
	Help         string      `json:"help,omitempty"`
	Group        string      `json:"group,omitempty"`
	Order        *int        `json:"order,omitempty"`
	Advanced     *bool       `json:"advanced,omitempty"`
	Sensitive    *bool       `json:"sensitive,omitempty"`
	Placeholder  string      `json:"placeholder,omitempty"`
	ItemTemplate interface{} `json:"itemTemplate,omitempty"`
}

// PluginUIMetadata 插件 UI 元数据
type PluginUIMetadata struct {
	ID            string            `json:"id"`
	Name          string            `json:"name,omitempty"`
	Description   string            `json:"description,omitempty"`
	ConfigUIHints map[string]UIHint `json:"configUiHints,omitempty"`
	ConfigSchema  interface{}       `json:"configSchema,omitempty"`
}

// ChannelUIMetadata 频道 UI 元数据
type ChannelUIMetadata struct {
	ID            string            `json:"id"`
	Label         string            `json:"label,omitempty"`
	Description   string            `json:"description,omitempty"`
	ConfigSchema  interface{}       `json:"configSchema,omitempty"`
	ConfigUIHints map[string]UIHint `json:"configUiHints,omitempty"`
}

// ValidateOpenAcosmiConfig 验证完整的 OpenAcosmi 配置
// 执行三层验证:
//  1. 结构体 validate 标签 (字段级约束)
//  2. 跨字段逻辑验证 (allow/alsoAllow 互斥等)
//  3. 语义验证 (如 browser profile 必须设 cdpPort 或 cdpUrl)
func ValidateOpenAcosmiConfig(cfg *types.OpenAcosmiConfig) []ValidationError {
	var errs []ValidationError

	// 第一层: struct tag 验证
	if tagErrs := ValidateConfig(cfg); len(tagErrs) > 0 {
		errs = append(errs, tagErrs...)
	}

	// 第二层: 跨字段验证
	errs = append(errs, validateCrossFieldRules(cfg)...)

	// 第三层: 深层约束验证 (枚举范围、数值范围)
	errs = append(errs, validateDeepConstraints(cfg)...)

	// 第四层: 语义验证 (avatar 路径、heartbeat target、agent 目录去重)
	errs = append(errs, validateIdentityAvatars(cfg)...)
	errs = append(errs, validateHeartbeatTargets(cfg)...)
	errs = append(errs, validateAgentDirDuplicates(cfg)...)

	return errs
}

// validateCrossFieldRules 跨字段验证规则
// 对应 Zod 中的 superRefine 回调
func validateCrossFieldRules(cfg *types.OpenAcosmiConfig) []ValidationError {
	var errs []ValidationError

	// Browser profile: 必须设 cdpPort 或 cdpUrl
	if cfg.Browser != nil && cfg.Browser.Profiles != nil {
		for name, profile := range cfg.Browser.Profiles {
			if profile != nil && profile.CdpPort == nil && profile.CdpURL == "" {
				errs = append(errs, ValidationError{
					Field:   "browser.profiles." + name,
					Tag:     "cdp_required",
					Message: "profile must set cdpPort or cdpUrl",
				})
			}
		}
	}

	// Agent tools: allow 和 alsoAllow 互斥
	if cfg.Agents != nil {
		for i := range cfg.Agents.List {
			agent := &cfg.Agents.List[i]
			if agent.Tools != nil {
				if ve := ValidateAllowAlsoAllowMutex(
					agent.Tools.Allow,
					agent.Tools.AlsoAllow,
				); ve != nil {
					ve.Field = fmt.Sprintf("agents.list[%d].tools.%s", i, ve.Field)
					errs = append(errs, *ve)
				}
			}
		}
	}

	// Broadcast agentId 交叉验证
	// 对应 TS zod-schema.ts L599-629 superRefine:
	// broadcast 中引用的 agentId 必须存在于 agents.list
	if cfg.Agents != nil && cfg.Broadcast != nil && cfg.Broadcast.Peers != nil {
		agentIDs := make(map[string]bool, len(cfg.Agents.List))
		for _, agent := range cfg.Agents.List {
			if agent.ID != "" {
				agentIDs[agent.ID] = true
			}
		}
		if len(agentIDs) > 0 {
			for peerId, ids := range cfg.Broadcast.Peers {
				for idx, agentId := range ids {
					if !agentIDs[agentId] {
						errs = append(errs, ValidationError{
							Field:   fmt.Sprintf("broadcast.%s[%d]", peerId, idx),
							Tag:     "unknown_agent",
							Value:   agentId,
							Message: fmt.Sprintf("Unknown agent id %q (not in agents.list)", agentId),
						})
					}
				}
			}
		}
	}

	return errs
}

// NewSchemaResponse 生成配置 schema 响应
func NewSchemaResponse(version string) *ConfigSchemaResponse {
	return &ConfigSchemaResponse{
		Schema:      generateConfigSchema(),
		Version:     version,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		UIHints:     buildUIHints(),
	}
}

// ----- 语义验证 (H7-3) -----

var (
	avatarDataRE   = regexp.MustCompile(`(?i)^data:`)
	avatarHTTPRE   = regexp.MustCompile(`(?i)^https?://`)
	avatarSchemeRE = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.-]*:`)
	windowsAbsRE   = regexp.MustCompile(`^[a-zA-Z]:[\\/]`)
)

// knownChannelIDs 已知频道 ID — 对应 TS CHAT_CHANNEL_ORDER
var knownChannelIDs = map[string]bool{
	"telegram":   true,
	"whatsapp":   true,
	"discord":    true,
	"googlechat": true,
	"slack":      true,
	"signal":     true,
	"imessage":   true,
	"feishu":     true,
	"dingtalk":   true,
	"wecom":      true,
}

// validateIdentityAvatars 验证 agent identity avatar 路径
// 对应 TS validateIdentityAvatar()
func validateIdentityAvatars(cfg *types.OpenAcosmiConfig) []ValidationError {
	if cfg.Agents == nil {
		return nil
	}
	var errs []ValidationError
	for i, agent := range cfg.Agents.List {
		if agent.Identity == nil {
			continue
		}
		avatar := strings.TrimSpace(agent.Identity.Avatar)
		if avatar == "" {
			continue
		}
		// data URI 和 HTTP(S) URL 允许
		if avatarDataRE.MatchString(avatar) || avatarHTTPRE.MatchString(avatar) {
			continue
		}
		// ~ 开头禁止
		if strings.HasPrefix(avatar, "~") {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("agents.list[%d].identity.avatar", i),
				Tag:     "avatar_path",
				Message: "identity.avatar must be a workspace-relative path, http(s) URL, or data URI",
			})
			continue
		}
		// 非 http(s) 的其他 scheme 禁止（Windows 绝对路径排除）
		if avatarSchemeRE.MatchString(avatar) && !windowsAbsRE.MatchString(avatar) {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("agents.list[%d].identity.avatar", i),
				Tag:     "avatar_path",
				Message: "identity.avatar must be a workspace-relative path, http(s) URL, or data URI",
			})
		}
	}
	return errs
}

// validateHeartbeatTargets 验证 heartbeat target 格式
// 对应 TS validateHeartbeatTarget()
func validateHeartbeatTargets(cfg *types.OpenAcosmiConfig) []ValidationError {
	var errs []ValidationError

	checkTarget := func(target string, path string) {
		trimmed := strings.TrimSpace(target)
		if trimmed == "" {
			return
		}
		normalized := strings.ToLower(trimmed)
		if normalized == "last" || normalized == "none" {
			return
		}
		if knownChannelIDs[normalized] {
			return
		}
		errs = append(errs, ValidationError{
			Field:   path,
			Tag:     "heartbeat_target",
			Value:   target,
			Message: fmt.Sprintf("unknown heartbeat target: %s", target),
		})
	}

	if cfg.Agents != nil {
		if cfg.Agents.Defaults != nil && cfg.Agents.Defaults.Heartbeat != nil {
			checkTarget(cfg.Agents.Defaults.Heartbeat.Target, "agents.defaults.heartbeat.target")
		}
		for i, agent := range cfg.Agents.List {
			if agent.Heartbeat != nil {
				checkTarget(agent.Heartbeat.Target, fmt.Sprintf("agents.list[%d].heartbeat.target", i))
			}
		}
	}
	return errs
}

// validateAgentDirDuplicates 检测重复 agent 工作目录
// 对应 TS findDuplicateAgentDirs()
func validateAgentDirDuplicates(cfg *types.OpenAcosmiConfig) []ValidationError {
	if cfg.Agents == nil || len(cfg.Agents.List) < 2 {
		return nil
	}
	seen := make(map[string]string) // dir → first agent ID
	var errs []ValidationError
	for _, agent := range cfg.Agents.List {
		dir := strings.TrimSpace(agent.AgentDir)
		if dir == "" {
			continue
		}
		if firstID, exists := seen[dir]; exists {
			errs = append(errs, ValidationError{
				Field:   "agents.list",
				Tag:     "duplicate_dir",
				Message: fmt.Sprintf("agents %q and %q share the same directory %q", firstID, agent.ID, dir),
			})
		} else {
			seen[dir] = agent.ID
		}
	}
	return errs
}

// buildUIHints 构建 UI 提示映射
// 对应 schema.ts 中的 buildBaseHints() + applySensitiveHints()
// 数据来源: schema_hints_data.go (自动生成自 TS schema.ts)
func buildUIHints() map[string]UIHint {
	hints := make(map[string]UIHint, len(fieldLabels)+len(groupLabels))

	// 第一遍: group hints (label + group + order)
	for group, label := range groupLabels {
		order := groupOrder[group]
		hints[group] = UIHint{
			Label: label,
			Group: label,
			Order: &order,
		}
	}

	// 第二遍: field labels
	for path, label := range fieldLabels {
		h := hints[path]
		h.Label = label
		hints[path] = h
	}

	// 第三遍: field help
	for path, help := range fieldHelp {
		h := hints[path]
		h.Help = help
		hints[path] = h
	}

	// 第四遍: field placeholders
	for path, placeholder := range fieldPlaceholders {
		h := hints[path]
		h.Placeholder = placeholder
		hints[path] = h
	}

	// 第五遍: sensitive 自动标记
	trueVal := true
	for key := range hints {
		if isSensitivePath(key) {
			h := hints[key]
			h.Sensitive = &trueVal
			hints[key] = h
		}
	}

	return hints
}
