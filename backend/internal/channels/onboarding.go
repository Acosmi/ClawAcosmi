package channels

// Onboarding 类型 — 继承自 src/channels/plugins/onboarding-types.ts (87L)
// + src/channels/plugins/onboarding/channel-access.ts (101L)

// ChannelAccessPolicy 频道访问策略
type ChannelAccessPolicy string

const (
	AccessPolicyAllowlist ChannelAccessPolicy = "allowlist"
	AccessPolicyOpen      ChannelAccessPolicy = "open"
	AccessPolicyDisabled  ChannelAccessPolicy = "disabled"
)

// OnboardingStatus 频道引导状态
type OnboardingStatus struct {
	Channel         ChannelID `json:"channel"`
	Configured      bool      `json:"configured"`
	StatusLines     []string  `json:"statusLines"`
	SelectionHint   string    `json:"selectionHint,omitempty"`
	QuickstartScore *int      `json:"quickstartScore,omitempty"`
}

// OnboardingResult 引导结果
type OnboardingResult struct {
	AccountID string `json:"accountId,omitempty"`
}

// SetupChannelsOptions 频道设置选项
type SetupChannelsOptions struct {
	AllowDisable           bool                 `json:"allowDisable,omitempty"`
	AccountIDs             map[ChannelID]string `json:"accountIds,omitempty"`
	ForceAllowFromChannels []ChannelID          `json:"forceAllowFromChannels,omitempty"`
	SkipStatusNote         bool                 `json:"skipStatusNote,omitempty"`
	SkipDmPolicyPrompt     bool                 `json:"skipDmPolicyPrompt,omitempty"`
	SkipConfirm            bool                 `json:"skipConfirm,omitempty"`
	QuickstartDefaults     bool                 `json:"quickstartDefaults,omitempty"`
	InitialSelection       []ChannelID          `json:"initialSelection,omitempty"`
}

// ParseAllowlistEntries 解析 allowlist 条目
func ParseAllowlistEntries(raw string) []string {
	var entries []string
	for _, part := range splitByCommaOrNewline(raw) {
		trimmed := trimString(part)
		if trimmed != "" {
			entries = append(entries, trimmed)
		}
	}
	return entries
}

// FormatAllowlistEntries 格式化 allowlist 条目
func FormatAllowlistEntries(entries []string) string {
	var filtered []string
	for _, e := range entries {
		t := trimString(e)
		if t != "" {
			filtered = append(filtered, t)
		}
	}
	return joinStrings(filtered, ", ")
}

// 内部辅助
func splitByCommaOrNewline(s string) []string {
	var result []string
	current := ""
	for _, ch := range s {
		if ch == ',' || ch == '\n' {
			result = append(result, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func trimString(s string) string {
	result := ""
	started := false
	end := 0
	for i, ch := range s {
		if ch != ' ' && ch != '\t' && ch != '\r' {
			if !started {
				started = true
			}
			end = i + 1
		}
	}
	if !started {
		return ""
	}
	for i, ch := range s {
		if i >= end {
			break
		}
		if started {
			result += string(ch)
		}
		if ch != ' ' && ch != '\t' && ch != '\r' {
			started = true
		}
	}
	return result
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}
