package channels

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/openacosmi/claw-acismi/pkg/i18n"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// Telegram 引导适配器 — 继承自 src/channels/plugins/onboarding/telegram.ts (357L)

// TelegramDmPolicyInfo Telegram DM 策略元数据
var TelegramDmPolicyInfo = struct {
	Label        string
	PolicyKey    string
	AllowFromKey string
}{
	Label:        "Telegram",
	PolicyKey:    "channels.telegram.dmPolicy",
	AllowFromKey: "channels.telegram.allowFrom",
}

// ParseTelegramAllowFromInput 解析 Telegram allowFrom 输入
func ParseTelegramAllowFromInput(raw string) []string {
	var result []string
	for _, part := range regexp.MustCompile(`[\n,;]+`).Split(raw, -1) {
		t := strings.TrimSpace(part)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

// ParseTelegramUserID 提取 Telegram user ID
func ParseTelegramUserID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	// 移除 @ 前缀
	if strings.HasPrefix(trimmed, "@") {
		return trimmed[1:]
	}
	// 纯数字
	if regexp.MustCompile(`^\d+$`).MatchString(trimmed) {
		return trimmed
	}
	// user:前缀
	prefixed := regexp.MustCompile(`(?i)^(user:|telegram:)`).ReplaceAllString(trimmed, "")
	if regexp.MustCompile(`^\d+$`).MatchString(prefixed) {
		return prefixed
	}
	// username
	if regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{4,}$`).MatchString(trimmed) {
		return trimmed
	}
	return ""
}

// BuildTelegramOnboardingStatus 构建 Telegram 引导状态
func BuildTelegramOnboardingStatus(configured bool) OnboardingStatus {
	statusStr := "needs token"
	if configured {
		statusStr = "configured"
	}
	// 对齐 TS: unconfigured=10 优先推荐新手，configured=1
	score := 10
	hint := "recommended · newcomer-friendly"
	if configured {
		hint = "recommended · configured"
		score = 1
	}
	return OnboardingStatus{
		Channel:    ChannelTelegram,
		Configured: configured,
		StatusLines: []string{
			fmt.Sprintf("Telegram: %s", statusStr),
		},
		SelectionHint:   hint,
		QuickstartScore: &score,
	}
}

// ---------- 交互向导 ----------

// ConfigureTelegramParams Telegram 配置参数。
type ConfigureTelegramParams struct {
	Cfg       *types.OpenAcosmiConfig
	Prompter  Prompter
	AccountID string
}

// ConfigureTelegramResult Telegram 配置结果。
type ConfigureTelegramResult struct {
	Cfg       *types.OpenAcosmiConfig
	AccountID string
}

// NoteTelegramTokenHelp 展示 Telegram bot token 帮助文本。
func NoteTelegramTokenHelp(prompter Prompter) {
	prompter.Note(strings.Join([]string{
		"1) Open Telegram → search @BotFather",
		"2) /newbot → set name and username",
		"3) Copy the bot token",
		"4) (Optional) /setprivacy → Disable (to read group messages)",
		"Docs: https://docs.openacosmi.dev/telegram",
	}, "\n"), i18n.Tp("onboard.ch.telegram.title"))
}

// NoteTelegramUserIDHelp 展示 Telegram User ID 帮助。
func NoteTelegramUserIDHelp(prompter Prompter) {
	prompter.Note(strings.Join([]string{
		"Find your Telegram user ID:",
		"- Send a message to @userinfobot",
		"- Or forward a message to @JsonDumpBot",
		"- Or use: getUpdates API after /start",
	}, "\n"), i18n.Tp("onboard.ch.telegram.title"))
}

// ConfigureTelegram 交互式 Telegram 频道配置向导。
// 对应 TS telegramOnboardingAdapter.configure (telegram.ts L175-340)。
func ConfigureTelegram(params ConfigureTelegramParams) (*ConfigureTelegramResult, error) {
	cfg := params.Cfg
	if cfg == nil {
		cfg = &types.OpenAcosmiConfig{}
	}
	prompter := params.Prompter
	accountID := params.AccountID
	if accountID == "" {
		accountID = DefaultAccountID
	}

	// 确保 channels.telegram 结构存在
	if cfg.Channels == nil {
		cfg.Channels = &types.ChannelsConfig{}
	}
	if cfg.Channels.Telegram == nil {
		cfg.Channels.Telegram = &types.TelegramConfig{}
	}
	enabledTrue := true
	cfg.Channels.Telegram.Enabled = &enabledTrue

	// 检测已有 token
	hasConfigToken := cfg.Channels.Telegram.BotToken != ""
	envToken := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	canUseEnv := accountID == DefaultAccountID && envToken != ""

	var botToken string

	if !hasConfigToken {
		NoteTelegramTokenHelp(prompter)
	}

	if canUseEnv && !hasConfigToken {
		keepEnv, err := prompter.Confirm(i18n.Tp("onboard.ch.telegram.env_found"), true)
		if err != nil {
			return nil, err
		}
		if !keepEnv {
			t, err := prompter.TextInput(i18n.Tp("onboard.ch.telegram.token"), "", "", func(v string) string {
				if strings.TrimSpace(v) == "" {
					return "Required"
				}
				return ""
			})
			if err != nil {
				return nil, err
			}
			botToken = strings.TrimSpace(t)
		}
	} else if hasConfigToken {
		keep, err := prompter.Confirm(i18n.Tp("onboard.ch.telegram.keep"), true)
		if err != nil {
			return nil, err
		}
		if !keep {
			t, err := prompter.TextInput(i18n.Tp("onboard.ch.telegram.token"), "", "", func(v string) string {
				if strings.TrimSpace(v) == "" {
					return "Required"
				}
				return ""
			})
			if err != nil {
				return nil, err
			}
			botToken = strings.TrimSpace(t)
		}
	} else {
		t, err := prompter.TextInput(i18n.Tp("onboard.ch.telegram.token"), "", "", func(v string) string {
			if strings.TrimSpace(v) == "" {
				return "Required"
			}
			return ""
		})
		if err != nil {
			return nil, err
		}
		botToken = strings.TrimSpace(t)
	}

	// 写入 token
	if botToken != "" {
		if accountID == DefaultAccountID {
			cfg.Channels.Telegram.BotToken = botToken
		} else {
			if cfg.Channels.Telegram.Accounts == nil {
				cfg.Channels.Telegram.Accounts = make(map[string]*types.TelegramAccountConfig)
			}
			acct := cfg.Channels.Telegram.Accounts[accountID]
			if acct == nil {
				acct = &types.TelegramAccountConfig{}
				e := true
				acct.Enabled = &e
			}
			acct.BotToken = botToken
			cfg.Channels.Telegram.Accounts[accountID] = acct
		}
	}

	// Group access 配置
	hasGroups := cfg.Channels.Telegram.Groups != nil && len(cfg.Channels.Telegram.Groups) > 0
	currentPolicy := cfg.Channels.Telegram.GroupPolicy
	if currentPolicy == "" {
		currentPolicy = types.GroupPolicy("allowlist")
	}
	accessConfig, err := PromptChannelAccessConfig(
		prompter, "Telegram groups",
		ChannelAccessPolicy(currentPolicy), nil,
		"-1001234567890, @mygroup",
		hasGroups,
	)
	if err != nil {
		return nil, err
	}
	if accessConfig != nil {
		cfg = SetTelegramGroupPolicy(cfg, accountID, string(accessConfig.Policy))
		if accessConfig.Policy == AccessPolicyAllowlist && len(accessConfig.Entries) > 0 {
			cfg = SetTelegramGroupAllowlist(cfg, accountID, accessConfig.Entries)
		}
	}

	return &ConfigureTelegramResult{Cfg: cfg, AccountID: accountID}, nil
}

// ---------- 配置写入辅助 ----------

// SetTelegramDmPolicy 设置 Telegram DM 策略。
func SetTelegramDmPolicy(cfg *types.OpenAcosmiConfig, policy types.DmPolicy) *types.OpenAcosmiConfig {
	ensureTelegramConfig(cfg)
	cfg.Channels.Telegram.DmPolicy = policy
	if policy == "open" {
		cfg.Channels.Telegram.AllowFrom = addWildcardInterface(cfg.Channels.Telegram.AllowFrom)
	}
	return cfg
}

// SetTelegramGroupPolicy 设置 Telegram 群组策略。
func SetTelegramGroupPolicy(cfg *types.OpenAcosmiConfig, accountID string, policy string) *types.OpenAcosmiConfig {
	ensureTelegramConfig(cfg)
	if accountID == DefaultAccountID {
		cfg.Channels.Telegram.GroupPolicy = types.GroupPolicy(policy)
	} else {
		if cfg.Channels.Telegram.Accounts == nil {
			cfg.Channels.Telegram.Accounts = make(map[string]*types.TelegramAccountConfig)
		}
		acct := cfg.Channels.Telegram.Accounts[accountID]
		if acct == nil {
			acct = &types.TelegramAccountConfig{}
			e := true
			acct.Enabled = &e
		}
		acct.GroupPolicy = types.GroupPolicy(policy)
		cfg.Channels.Telegram.Accounts[accountID] = acct
	}
	return cfg
}

// SetTelegramGroupAllowlist 设置 Telegram 群组允许列表。
func SetTelegramGroupAllowlist(cfg *types.OpenAcosmiConfig, accountID string, groupKeys []string) *types.OpenAcosmiConfig {
	ensureTelegramConfig(cfg)
	groups := make(map[string]*types.TelegramGroupConfig)
	for _, key := range groupKeys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		e := true
		groups[trimmed] = &types.TelegramGroupConfig{Enabled: &e}
	}
	if accountID == DefaultAccountID {
		cfg.Channels.Telegram.Groups = groups
	} else {
		if cfg.Channels.Telegram.Accounts == nil {
			cfg.Channels.Telegram.Accounts = make(map[string]*types.TelegramAccountConfig)
		}
		acct := cfg.Channels.Telegram.Accounts[accountID]
		if acct == nil {
			acct = &types.TelegramAccountConfig{}
			e := true
			acct.Enabled = &e
		}
		acct.Groups = groups
		cfg.Channels.Telegram.Accounts[accountID] = acct
	}
	return cfg
}

// SetTelegramAllowFrom 设置 Telegram DM allowFrom。
func SetTelegramAllowFrom(cfg *types.OpenAcosmiConfig, accountID string, allowFrom []string) *types.OpenAcosmiConfig {
	ensureTelegramConfig(cfg)
	ifaces := make([]interface{}, len(allowFrom))
	for i, v := range allowFrom {
		ifaces[i] = v
	}
	if accountID == DefaultAccountID {
		cfg.Channels.Telegram.AllowFrom = ifaces
	} else {
		if cfg.Channels.Telegram.Accounts == nil {
			cfg.Channels.Telegram.Accounts = make(map[string]*types.TelegramAccountConfig)
		}
		acct := cfg.Channels.Telegram.Accounts[accountID]
		if acct == nil {
			acct = &types.TelegramAccountConfig{}
			e := true
			acct.Enabled = &e
		}
		acct.AllowFrom = ifaces
		cfg.Channels.Telegram.Accounts[accountID] = acct
	}
	return cfg
}

// extractTelegramBotToken 从配置中提取 bot token（不依赖 telegram 包避免循环导入）。
func extractTelegramBotToken(cfg *types.OpenAcosmiConfig, accountID string) string {
	if cfg == nil || cfg.Channels == nil || cfg.Channels.Telegram == nil {
		return ""
	}
	tg := cfg.Channels.Telegram
	if accountID != "" && accountID != DefaultAccountID && tg.Accounts != nil {
		if acct, ok := tg.Accounts[accountID]; ok && acct != nil && acct.BotToken != "" {
			return acct.BotToken
		}
	}
	if tg.BotToken != "" {
		return tg.BotToken
	}
	if accountID == DefaultAccountID || accountID == "" {
		return strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	}
	return ""
}

// resolveTelegramUserIDViaAPI 通过 Telegram API 解析 username → user ID。
// 对齐 TS: promptTelegramAllowFrom 中 resolveTelegramUserId 的 getChat API 调用。
// 纯数字直接返回；无 token 或 API 失败时回退到本地解析。
func resolveTelegramUserIDViaAPI(token, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	// 去除 telegram:/tg: 前缀
	stripped := regexp.MustCompile(`(?i)^(telegram:|tg:)`).ReplaceAllString(trimmed, "")
	stripped = strings.TrimSpace(stripped)
	// 纯数字直接返回
	if regexp.MustCompile(`^\d+$`).MatchString(stripped) {
		return stripped
	}
	if token == "" {
		// 对齐 TS: 无 token 时返回空字符串，强制用户提供数字 ID
		return ""
	}
	// 通过 API 解析 username
	username := stripped
	if !strings.HasPrefix(username, "@") {
		username = "@" + username
	}
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getChat?chat_id=%s",
		token, url.QueryEscape(username))
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	defer resp.Body.Close()
	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			ID json.Number `json:"id"`
		} `json:"result"`
	}
	if json.NewDecoder(resp.Body).Decode(&result) != nil || !result.OK {
		return ""
	}
	return result.Result.ID.String()
}

// PromptTelegramAllowFrom 交互式 Telegram allowFrom 输入。
// 对齐 TS: 通过 Telegram API 解析 username，解析失败时提示用户重试。
func PromptTelegramAllowFrom(cfg *types.OpenAcosmiConfig, prompter Prompter, accountID string) (*types.OpenAcosmiConfig, error) {
	if accountID == "" {
		accountID = DefaultAccountID
	}
	NoteTelegramUserIDHelp(prompter)

	token := extractTelegramBotToken(cfg, accountID)
	if token == "" {
		prompter.Note("Telegram token missing; username lookup is unavailable.", i18n.Tp("onboard.ch.telegram.title"))
	}

	var resolvedIDs []string
	for resolvedIDs == nil {
		entry, err := prompter.TextInput(
			"Telegram allowFrom (username or user id)",
			"@username",
			"",
			func(v string) string {
				if strings.TrimSpace(v) == "" {
					return "Required"
				}
				return ""
			},
		)
		if err != nil {
			return cfg, err
		}
		parts := ParseTelegramAllowFromInput(entry)
		var resolved []string
		var unresolved []string
		for _, part := range parts {
			id := resolveTelegramUserIDViaAPI(token, part)
			if id != "" {
				resolved = append(resolved, id)
			} else {
				unresolved = append(unresolved, part)
			}
		}
		if len(unresolved) > 0 {
			prompter.Note(
				fmt.Sprintf("Could not resolve: %s. Use @username or numeric id.", strings.Join(unresolved, ", ")),
				i18n.Tp("onboard.ch.telegram.title"),
			)
			continue
		}
		resolvedIDs = resolved
	}

	// 对齐 TS: 合并已有 allowFrom + 新解析 ID
	var existing []string
	if cfg.Channels != nil && cfg.Channels.Telegram != nil {
		var allowFrom []interface{}
		if accountID != DefaultAccountID && cfg.Channels.Telegram.Accounts != nil {
			if acct, ok := cfg.Channels.Telegram.Accounts[accountID]; ok && acct != nil {
				allowFrom = acct.AllowFrom
			}
		} else {
			allowFrom = cfg.Channels.Telegram.AllowFrom
		}
		for _, item := range allowFrom {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					existing = append(existing, s)
				}
			}
		}
	}
	merged := append(existing, resolvedIDs...)
	unique := UniqueStrings(merged)
	return SetTelegramAllowFrom(cfg, accountID, unique), nil
}

// DisableTelegram 禁用 Telegram 频道。
func DisableTelegram(cfg *types.OpenAcosmiConfig) *types.OpenAcosmiConfig {
	if cfg.Channels != nil && cfg.Channels.Telegram != nil {
		e := false
		cfg.Channels.Telegram.Enabled = &e
	}
	return cfg
}

func ensureTelegramConfig(cfg *types.OpenAcosmiConfig) {
	if cfg.Channels == nil {
		cfg.Channels = &types.ChannelsConfig{}
	}
	if cfg.Channels.Telegram == nil {
		cfg.Channels.Telegram = &types.TelegramConfig{}
	}
	e := true
	cfg.Channels.Telegram.Enabled = &e
}
