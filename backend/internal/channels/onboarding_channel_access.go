package channels

// onboarding_channel_access.go — 频道访问策略交互 + 本地 Prompter 接口
// TS 对照: src/channels/plugins/onboarding/channel-access.ts (101L)
//
// 注: channels 包不能导入 tui（循环依赖: channels → tui → gateway → channels）
// 因此在此定义 Prompter/PromptOption 接口，由调用方传入 tui 实例即可。

import (
	"strings"

	"github.com/Acosmi/ClawAcosmi/pkg/i18n"
)

// ---------- 接口定义（镜像 tui.WizardPrompter / tui.PromptOption，避免循环导入） ----------

// Prompter 频道引导交互接口。
// 调用方传入 tui.WizardPrompter 实例即可满足此接口。
type Prompter interface {
	Intro(title string)
	Outro(message string)
	Note(message, title string)
	Select(message string, options []PromptOption, initialValue string) (string, error)
	MultiSelect(message string, options []PromptOption, initialValues []string) ([]string, error)
	TextInput(message, placeholder, initial string, validate func(string) string) (string, error)
	Confirm(message string, initial bool) (bool, error)
}

// PromptOption select/multiselect 选项。
type PromptOption struct {
	Value string
	Label string
	Hint  string
}

// ---------- Channel Access 类型 ----------

// AccessConfig 频道访问配置结果。
type AccessConfig struct {
	Policy  ChannelAccessPolicy
	Entries []string
}

// ---------- 交互函数 ----------

// PromptChannelAccessPolicy 交互式选择频道访问策略。
// 对应 TS promptChannelAccessPolicy (channel-access.ts L19-41)。
func PromptChannelAccessPolicy(
	prompter Prompter,
	label string,
	currentPolicy ChannelAccessPolicy,
	allowOpen bool,
	allowDisabled bool,
) (ChannelAccessPolicy, error) {
	options := []PromptOption{
		{Value: string(AccessPolicyAllowlist), Label: "Allowlist (recommended)"},
	}
	if allowOpen {
		options = append(options, PromptOption{Value: string(AccessPolicyOpen), Label: "Open (allow all channels)"})
	}
	if allowDisabled {
		options = append(options, PromptOption{Value: string(AccessPolicyDisabled), Label: "Disabled (block all channels)"})
	}
	initial := string(currentPolicy)
	if initial == "" {
		initial = string(AccessPolicyAllowlist)
	}
	selected, err := prompter.Select(i18n.Tf("onboard.ch.access.title", label), options, initial)
	if err != nil {
		return "", err
	}
	return ChannelAccessPolicy(selected), nil
}

// PromptChannelAllowlist 交互式输入频道允许列表。
// 对应 TS promptChannelAllowlist (channel-access.ts L43-59)。
func PromptChannelAllowlist(
	prompter Prompter,
	label string,
	currentEntries []string,
	placeholder string,
) ([]string, error) {
	initialValue := ""
	if len(currentEntries) > 0 {
		initialValue = FormatAllowlistEntries(currentEntries)
	}
	raw, err := prompter.TextInput(
		i18n.Tp("onboard.ch.access.input"),
		placeholder,
		initialValue,
		nil,
	)
	if err != nil {
		return nil, err
	}
	return ParseAllowlistEntries(raw), nil
}

// PromptChannelAccessConfig 交互式配置频道访问（策略 + 允许列表）。
// 对应 TS promptChannelAccessConfig (channel-access.ts L61-100)。
func PromptChannelAccessConfig(
	prompter Prompter,
	label string,
	currentPolicy ChannelAccessPolicy,
	currentEntries []string,
	placeholder string,
	updatePrompt bool,
) (*AccessConfig, error) {
	hasEntries := len(currentEntries) > 0
	shouldPrompt := !hasEntries

	message := i18n.Tf("onboard.ch.access.confirm", label)
	wants, err := prompter.Confirm(message, shouldPrompt)
	if err != nil {
		return nil, err
	}
	if !wants {
		return nil, nil
	}

	policy, err := PromptChannelAccessPolicy(prompter, label, currentPolicy, true, true)
	if err != nil {
		return nil, err
	}
	if policy != AccessPolicyAllowlist {
		return &AccessConfig{Policy: policy, Entries: nil}, nil
	}

	entries, err := PromptChannelAllowlist(prompter, label, currentEntries, placeholder)
	if err != nil {
		return nil, err
	}
	return &AccessConfig{Policy: policy, Entries: entries}, nil
}

// ---------- 辅助函数 ----------

// UniqueStrings 去重字符串切片（保持顺序）。
func UniqueStrings(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			result = append(result, trimmed)
		}
	}
	return result
}
