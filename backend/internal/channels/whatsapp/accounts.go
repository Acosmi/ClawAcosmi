package whatsapp

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openacosmi/claw-acismi/internal/config"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// WhatsApp 账户解析 — 继承自 src/web/accounts.ts (177L)

// resolveUserPath 展开 ~ 前缀的用户路径
func resolveUserPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// HasWebCredsSync 检查指定目录是否包含有效的 creds.json
func HasWebCredsSync(authDir string) bool {
	credsPath := filepath.Join(authDir, "creds.json")
	info, err := os.Stat(credsPath)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Size() > 1
}

// defaultAccountID 默认账户 ID（与 channels.DefaultAccountID 一致，避免循环导入）
const defaultAccountID = "default"

// ResolvedWhatsAppAccount 解析后的 WhatsApp 账户配置（合并根级+账户级）
type ResolvedWhatsAppAccount struct {
	AccountID        string
	Name             string
	Enabled          bool
	SendReadReceipts bool
	MessagePrefix    string
	AuthDir          string
	IsLegacyAuthDir  bool
	SelfChatMode     bool
	AllowFrom        []string
	GroupAllowFrom   []string
	GroupPolicy      types.GroupPolicy
	GroupSilentToken string // silent_token 模式的全局激活词
	DmPolicy         types.DmPolicy
	TextChunkLimit   *int
	ChunkMode        string // "length"|"newline"
	MediaMaxMB       *int
	BlockStreaming   *bool
	AckReaction      *types.WhatsAppAckReactionConfig
	Groups           map[string]*types.WhatsAppGroupConfig
	DebounceMs       *int
}

// ResolveSilentToken 获取账户的 silent_token 激活词
// 优先使用账户级 GroupSilentToken，默认为空字符串（需配置才生效）
func ResolveSilentToken(account ResolvedWhatsAppAccount) string {
	return account.GroupSilentToken
}

// listConfiguredAccountIds 获取配置中定义的账户 ID 列表
func listConfiguredAccountIds(cfg *types.OpenAcosmiConfig) []string {
	if cfg.Channels == nil || cfg.Channels.WhatsApp == nil {
		return nil
	}
	accounts := cfg.Channels.WhatsApp.Accounts
	if len(accounts) == 0 {
		return nil
	}
	var ids []string
	for id := range accounts {
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// ListWhatsAppAuthDirs 列出所有 WhatsApp 认证目录
func ListWhatsAppAuthDirs(cfg *types.OpenAcosmiConfig) []string {
	oauthDir := config.ResolveOAuthDir()
	whatsappDir := filepath.Join(oauthDir, "whatsapp")

	seen := make(map[string]bool)
	var dirs []string
	addDir := func(d string) {
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	addDir(oauthDir)
	addDir(filepath.Join(whatsappDir, defaultAccountID))

	for _, accountID := range listConfiguredAccountIds(cfg) {
		result := ResolveWhatsAppAuthDir(cfg, accountID)
		addDir(result.AuthDir)
	}

	// 扫描 whatsapp/ 下的所有子目录
	entries, err := os.ReadDir(whatsappDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				addDir(filepath.Join(whatsappDir, entry.Name()))
			}
		}
	}

	return dirs
}

// HasAnyWhatsAppAuth 检查是否有任何 WhatsApp 认证
func HasAnyWhatsAppAuth(cfg *types.OpenAcosmiConfig) bool {
	for _, authDir := range ListWhatsAppAuthDirs(cfg) {
		if HasWebCredsSync(authDir) {
			return true
		}
	}
	return false
}

// ListWhatsAppAccountIds 列出所有账户 ID（无配置时返回 [default]）
func ListWhatsAppAccountIds(cfg *types.OpenAcosmiConfig) []string {
	ids := listConfiguredAccountIds(cfg)
	if len(ids) == 0 {
		return []string{defaultAccountID}
	}
	sort.Strings(ids)
	return ids
}

// ResolveDefaultWhatsAppAccountId 解析默认账户 ID
func ResolveDefaultWhatsAppAccountId(cfg *types.OpenAcosmiConfig) string {
	ids := ListWhatsAppAccountIds(cfg)
	for _, id := range ids {
		if id == defaultAccountID {
			return defaultAccountID
		}
	}
	if len(ids) > 0 {
		return ids[0]
	}
	return defaultAccountID
}

// resolveAccountConfig 获取指定账户的配置
func resolveAccountConfig(cfg *types.OpenAcosmiConfig, accountID string) *types.WhatsAppAccountConfig {
	if cfg.Channels == nil || cfg.Channels.WhatsApp == nil {
		return nil
	}
	accounts := cfg.Channels.WhatsApp.Accounts
	if len(accounts) == 0 {
		return nil
	}
	return accounts[accountID]
}

// normalizeAccountID 规范化账户 ID
func normalizeAccountID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultAccountID
	}
	return strings.ToLower(trimmed)
}

// resolveDefaultAuthDir 获取默认认证目录
func resolveDefaultAuthDir(accountID string) string {
	return filepath.Join(config.ResolveOAuthDir(), "whatsapp", normalizeAccountID(accountID))
}

// resolveLegacyAuthDir 获取旧版认证目录（Baileys 旧版 creds 存放位置）
func resolveLegacyAuthDir() string {
	return config.ResolveOAuthDir()
}

// legacyAuthExists 检查旧版认证是否存在
func legacyAuthExists(authDir string) bool {
	_, err := os.Stat(filepath.Join(authDir, "creds.json"))
	return err == nil
}

// AuthDirResult 认证目录解析结果
type AuthDirResult struct {
	AuthDir  string
	IsLegacy bool
}

// ResolveWhatsAppAuthDir 解析 WhatsApp 认证目录
// 支持账户级 authDir 覆盖、legacy 目录回退
func ResolveWhatsAppAuthDir(cfg *types.OpenAcosmiConfig, accountID string) AuthDirResult {
	id := strings.TrimSpace(accountID)
	if id == "" {
		id = defaultAccountID
	}
	acctCfg := resolveAccountConfig(cfg, id)
	if acctCfg != nil {
		configured := strings.TrimSpace(acctCfg.AuthDir)
		if configured != "" {
			return AuthDirResult{AuthDir: resolveUserPath(configured), IsLegacy: false}
		}
	}

	defaultDir := resolveDefaultAuthDir(id)
	if id == defaultAccountID {
		legacyDir := resolveLegacyAuthDir()
		if legacyAuthExists(legacyDir) && !legacyAuthExists(defaultDir) {
			return AuthDirResult{AuthDir: legacyDir, IsLegacy: true}
		}
	}

	return AuthDirResult{AuthDir: defaultDir, IsLegacy: false}
}

// ResolveWhatsAppAccount 解析完整的 WhatsApp 账户配置
// 实现账户级 ← 根级配置的级联合并
func ResolveWhatsAppAccount(cfg *types.OpenAcosmiConfig, accountID string) ResolvedWhatsAppAccount {
	var rootCfg *types.WhatsAppConfig
	if cfg.Channels != nil {
		rootCfg = cfg.Channels.WhatsApp
	}

	id := strings.TrimSpace(accountID)
	if id == "" {
		id = ResolveDefaultWhatsAppAccountId(cfg)
	}
	acctCfg := resolveAccountConfig(cfg, id)

	enabled := true
	if acctCfg != nil && acctCfg.Enabled != nil && !*acctCfg.Enabled {
		enabled = false
	}

	authResult := ResolveWhatsAppAuthDir(cfg, id)

	// sendReadReceipts: 账户级 → 根级 → true
	sendReadReceipts := true
	if acctCfg != nil && acctCfg.SendReadReceipts != nil {
		sendReadReceipts = *acctCfg.SendReadReceipts
	} else if rootCfg != nil && rootCfg.SendReadReceipts != nil {
		sendReadReceipts = *rootCfg.SendReadReceipts
	}

	// messagePrefix: 账户级 → 根级 → messages 全局级
	messagePrefix := ""
	if acctCfg != nil && acctCfg.MessagePrefix != "" {
		messagePrefix = acctCfg.MessagePrefix
	} else if rootCfg != nil && rootCfg.MessagePrefix != "" {
		messagePrefix = rootCfg.MessagePrefix
	} else if cfg.Messages != nil {
		messagePrefix = cfg.Messages.MessagePrefix
	}

	// selfChatMode
	selfChatMode := false
	if acctCfg != nil && acctCfg.SelfChatMode != nil {
		selfChatMode = *acctCfg.SelfChatMode
	} else if rootCfg != nil && rootCfg.SelfChatMode != nil {
		selfChatMode = *rootCfg.SelfChatMode
	}

	result := ResolvedWhatsAppAccount{
		AccountID:        id,
		Enabled:          enabled,
		SendReadReceipts: sendReadReceipts,
		MessagePrefix:    messagePrefix,
		AuthDir:          authResult.AuthDir,
		IsLegacyAuthDir:  authResult.IsLegacy,
		SelfChatMode:     selfChatMode,
	}

	if acctCfg != nil && acctCfg.Name != "" {
		result.Name = strings.TrimSpace(acctCfg.Name)
	}

	// dmPolicy: 账户级 → 根级
	if acctCfg != nil && acctCfg.DmPolicy != "" {
		result.DmPolicy = acctCfg.DmPolicy
	} else if rootCfg != nil && rootCfg.DmPolicy != "" {
		result.DmPolicy = rootCfg.DmPolicy
	}

	// allowFrom
	if acctCfg != nil && len(acctCfg.AllowFrom) > 0 {
		result.AllowFrom = acctCfg.AllowFrom
	} else if rootCfg != nil && len(rootCfg.AllowFrom) > 0 {
		result.AllowFrom = rootCfg.AllowFrom
	}

	// groupAllowFrom
	if acctCfg != nil && len(acctCfg.GroupAllowFrom) > 0 {
		result.GroupAllowFrom = acctCfg.GroupAllowFrom
	} else if rootCfg != nil && len(rootCfg.GroupAllowFrom) > 0 {
		result.GroupAllowFrom = rootCfg.GroupAllowFrom
	}

	// groupPolicy
	if acctCfg != nil && acctCfg.GroupPolicy != "" {
		result.GroupPolicy = acctCfg.GroupPolicy
	} else if rootCfg != nil && rootCfg.GroupPolicy != "" {
		result.GroupPolicy = rootCfg.GroupPolicy
	}

	// groupSilentToken
	if acctCfg != nil && acctCfg.GroupSilentToken != "" {
		result.GroupSilentToken = acctCfg.GroupSilentToken
	} else if rootCfg != nil && rootCfg.GroupSilentToken != "" {
		result.GroupSilentToken = rootCfg.GroupSilentToken
	}

	// textChunkLimit
	if acctCfg != nil && acctCfg.TextChunkLimit != nil {
		result.TextChunkLimit = acctCfg.TextChunkLimit
	} else if rootCfg != nil && rootCfg.TextChunkLimit != nil {
		result.TextChunkLimit = rootCfg.TextChunkLimit
	}

	// chunkMode
	if acctCfg != nil && acctCfg.ChunkMode != "" {
		result.ChunkMode = acctCfg.ChunkMode
	} else if rootCfg != nil && rootCfg.ChunkMode != "" {
		result.ChunkMode = rootCfg.ChunkMode
	}

	// mediaMaxMB
	if acctCfg != nil && acctCfg.MediaMaxMB != nil {
		result.MediaMaxMB = acctCfg.MediaMaxMB
	} else if rootCfg != nil && rootCfg.MediaMaxMB != nil {
		result.MediaMaxMB = rootCfg.MediaMaxMB
	}

	// blockStreaming
	if acctCfg != nil && acctCfg.BlockStreaming != nil {
		result.BlockStreaming = acctCfg.BlockStreaming
	} else if rootCfg != nil && rootCfg.BlockStreaming != nil {
		result.BlockStreaming = rootCfg.BlockStreaming
	}

	// ackReaction
	if acctCfg != nil && acctCfg.AckReaction != nil {
		result.AckReaction = acctCfg.AckReaction
	} else if rootCfg != nil && rootCfg.AckReaction != nil {
		result.AckReaction = rootCfg.AckReaction
	}

	// groups
	if acctCfg != nil && len(acctCfg.Groups) > 0 {
		result.Groups = acctCfg.Groups
	} else if rootCfg != nil && len(rootCfg.Groups) > 0 {
		result.Groups = rootCfg.Groups
	}

	// debounceMs
	if acctCfg != nil && acctCfg.DebounceMs != nil {
		result.DebounceMs = acctCfg.DebounceMs
	} else if rootCfg != nil && rootCfg.DebounceMs != nil {
		result.DebounceMs = rootCfg.DebounceMs
	}

	return result
}

// ListEnabledWhatsAppAccounts 列出所有启用的 WhatsApp 账户
func ListEnabledWhatsAppAccounts(cfg *types.OpenAcosmiConfig) []ResolvedWhatsAppAccount {
	var accounts []ResolvedWhatsAppAccount
	for _, id := range ListWhatsAppAccountIds(cfg) {
		acct := ResolveWhatsAppAccount(cfg, id)
		if acct.Enabled {
			accounts = append(accounts, acct)
		}
	}
	return accounts
}
