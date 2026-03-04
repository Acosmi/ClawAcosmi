package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// Slack 监控上下文 — 继承自 src/slack/monitor/context.ts (429L)
// Phase 9 实现：完整上下文管理（API 回填、dedup、策略门控）。

// SlackMonitorContext 监控上下文（集中管理配置、状态、工具函数）。
type SlackMonitorContext struct {
	mu sync.RWMutex

	// 基本配置
	AccountID string
	Account   ResolvedSlackAccount
	BotUserID string
	TeamID    string
	APIAppID  string
	Client    *SlackWebClient
	Mode      MonitorSlackMode
	CFG       *types.OpenAcosmiConfig

	// DI 依赖
	Deps *SlackMonitorDeps

	// 过滤配置
	AllowFrom      []string
	ChannelConfigs map[string]*types.SlackChannelConfig
	GroupPolicy    string
	RequireMention bool
	AllowBots      bool

	// DM 配置
	DMEnabled       bool
	DMPolicy        string // "open" | "pairing" | "allowlist"
	GroupDMEnabled  bool
	GroupDMChannels []string

	// 会话/历史配置
	HistoryLimit int
	TextLimit    int
	SessionScope string // "per-sender" | "per-channel" | etc
	MainKey      string

	// 反应/行为配置
	ReactionMode        string // "off" | "own" | "all" | "allowlist"
	ReactionAllowlist   []interface{}
	ReplyToMode         string // "off" | "first" | "all"
	AckReactionScope    string
	MediaMaxBytes       int
	RemoveAckAfterReply bool
	ThreadHistoryScope  string // "thread" | "channel"
	ThreadInheritParent bool

	// 斜杠命令配置
	SlashCommand    *types.SlackSlashCommandConfig
	UseAccessGroups bool // TS: cfg.commands?.useAccessGroups !== false — default true

	// 缓存
	channelCache map[string]*slackChannelInfo
	userCache    map[string]*slackUserInfo
	seenMessages map[string]int64 // dedup: key → timestamp
}

// slackChannelInfo 缓存的频道信息
type slackChannelInfo struct {
	Name    string
	Type    SlackChannelType
	Topic   string
	Purpose string
}

// slackUserInfo 缓存的用户信息
type slackUserInfo struct {
	Name string
}

// NewSlackMonitorContext 创建监控上下文。
func NewSlackMonitorContext(cfg *types.OpenAcosmiConfig, accountID string, deps *SlackMonitorDeps) *SlackMonitorContext {
	account := ResolveSlackAccount(cfg, accountID)
	client := NewSlackWebClient(account.BotToken)

	groupPolicy := "allowlist"
	if account.GroupPolicy != "" {
		groupPolicy = string(account.GroupPolicy)
	}

	requireMention := true
	if account.Config.RequireMention != nil {
		requireMention = *account.Config.RequireMention
	}

	allowBots := false
	if account.Config.AllowBots != nil {
		allowBots = *account.Config.AllowBots
	}

	// DM 配置
	dmEnabled := true
	dmPolicy := "pairing"
	var groupDMEnabled bool
	var groupDMChannels []string
	if account.DM != nil {
		if account.DM.Enabled != nil {
			dmEnabled = *account.DM.Enabled
		}
		if account.DM.Policy != "" {
			dmPolicy = string(account.DM.Policy)
		}
		if account.DM.GroupEnabled != nil {
			groupDMEnabled = *account.DM.GroupEnabled
		}
	}

	// 历史限制
	historyLimit := 20
	if account.Config.HistoryLimit != nil {
		historyLimit = *account.Config.HistoryLimit
	}

	// 文本限制
	textLimit := 4000
	if account.TextChunkLimit != nil {
		textLimit = *account.TextChunkLimit
	}

	// 反应模式
	reactionMode := "own"
	if account.ReactionNotifications != "" {
		reactionMode = string(account.ReactionNotifications)
	}

	// 媒体限制
	mediaMaxBytes := 20 * 1024 * 1024
	if account.MediaMaxMB != nil {
		mediaMaxBytes = *account.MediaMaxMB * 1024 * 1024
	}

	replyToMode := "off"
	if account.ReplyToMode != "" {
		replyToMode = string(account.ReplyToMode)
	}

	ackReactionScope := "group-mentions"
	if cfg.Messages != nil && cfg.Messages.AckReactionScope != "" {
		ackReactionScope = string(cfg.Messages.AckReactionScope)
	}

	removeAckAfterReply := false
	if cfg.Messages != nil && cfg.Messages.RemoveAckAfterReply != nil {
		removeAckAfterReply = *cfg.Messages.RemoveAckAfterReply
	}

	threadHistoryScope := "thread"
	threadInheritParent := false
	if account.Config.Thread != nil {
		if account.Config.Thread.HistoryScope != "" {
			threadHistoryScope = account.Config.Thread.HistoryScope
		}
		if account.Config.Thread.InheritParent != nil {
			threadInheritParent = *account.Config.Thread.InheritParent
		}
	}

	sessionScope := "per-sender"
	if cfg.Session != nil && cfg.Session.Scope != "" {
		sessionScope = string(cfg.Session.Scope)
	}
	mainKey := ""
	if cfg.Session != nil && cfg.Session.MainKey != "" {
		mainKey = cfg.Session.MainKey
	}

	// AllowFrom
	var allowFrom []string
	if account.DM != nil && len(account.DM.AllowFrom) > 0 {
		allowFrom = NormalizeAllowList(account.DM.AllowFrom)
	}

	return &SlackMonitorContext{
		AccountID:           accountID,
		Account:             account,
		Client:              client,
		CFG:                 cfg,
		Deps:                deps,
		ChannelConfigs:      account.Channels,
		GroupPolicy:         groupPolicy,
		RequireMention:      requireMention,
		AllowBots:           allowBots,
		DMEnabled:           dmEnabled,
		DMPolicy:            dmPolicy,
		GroupDMEnabled:      groupDMEnabled,
		GroupDMChannels:     groupDMChannels,
		AllowFrom:           allowFrom,
		HistoryLimit:        historyLimit,
		TextLimit:           textLimit,
		SessionScope:        sessionScope,
		MainKey:             mainKey,
		ReactionMode:        reactionMode,
		ReactionAllowlist:   account.ReactionAllowlist,
		ReplyToMode:         replyToMode,
		AckReactionScope:    ackReactionScope,
		MediaMaxBytes:       mediaMaxBytes,
		RemoveAckAfterReply: removeAckAfterReply,
		ThreadHistoryScope:  threadHistoryScope,
		ThreadInheritParent: threadInheritParent,
		SlashCommand:        account.SlashCommand,
		UseAccessGroups:     resolveUseAccessGroups(cfg),
		channelCache:        make(map[string]*slackChannelInfo),
		userCache:           make(map[string]*slackUserInfo),
		seenMessages:        make(map[string]int64),
	}
}

// ResolveChannelName 解析频道名（从缓存或 API 回填）。
func (ctx *SlackMonitorContext) ResolveChannelName(channelID string) string {
	info := ctx.resolveChannelInfo(channelID)
	if info != nil && info.Name != "" {
		return info.Name
	}
	return channelID
}

// resolveChannelInfo 获取完整频道信息（缓存 + API 回填）。
func (ctx *SlackMonitorContext) resolveChannelInfo(channelID string) *slackChannelInfo {
	ctx.mu.RLock()
	info, ok := ctx.channelCache[channelID]
	ctx.mu.RUnlock()
	if ok {
		return info
	}

	// API 回填: conversations.info
	raw, err := ctx.Client.APICall(context.Background(), "conversations.info", map[string]interface{}{
		"channel": channelID,
	})
	if err != nil {
		return nil
	}

	var resp struct {
		Channel struct {
			Name      string `json:"name"`
			IsIM      bool   `json:"is_im"`
			IsMPIM    bool   `json:"is_mpim"`
			IsChannel bool   `json:"is_channel"`
			IsGroup   bool   `json:"is_group"`
			Topic     struct {
				Value string `json:"value"`
			} `json:"topic"`
			Purpose struct {
				Value string `json:"value"`
			} `json:"purpose"`
		} `json:"channel"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}

	chType := SlackChannelTypeChannel
	switch {
	case resp.Channel.IsIM:
		chType = SlackChannelTypeIM
	case resp.Channel.IsMPIM:
		chType = SlackChannelTypeMPIM
	case resp.Channel.IsGroup:
		chType = SlackChannelTypeGroup
	}

	info = &slackChannelInfo{
		Name:    resp.Channel.Name,
		Type:    chType,
		Topic:   resp.Channel.Topic.Value,
		Purpose: resp.Channel.Purpose.Value,
	}

	ctx.mu.Lock()
	ctx.channelCache[channelID] = info
	ctx.mu.Unlock()

	return info
}

// CacheChannelName 缓存频道名。
func (ctx *SlackMonitorContext) CacheChannelName(channelID, name string) {
	ctx.mu.Lock()
	if existing, ok := ctx.channelCache[channelID]; ok {
		existing.Name = name
	} else {
		ctx.channelCache[channelID] = &slackChannelInfo{Name: name}
	}
	ctx.mu.Unlock()
}

// ResolveUserName 解析用户名（从缓存或 API 回填）。
func (ctx *SlackMonitorContext) ResolveUserName(userID string) string {
	ctx.mu.RLock()
	info, ok := ctx.userCache[userID]
	ctx.mu.RUnlock()
	if ok && info.Name != "" {
		return info.Name
	}

	// API 回填: users.info
	raw, err := ctx.Client.APICall(context.Background(), "users.info", map[string]interface{}{
		"user": userID,
	})
	if err != nil {
		return userID
	}

	var resp struct {
		User struct {
			Name    string `json:"name"`
			Profile struct {
				DisplayName string `json:"display_name"`
				RealName    string `json:"real_name"`
			} `json:"profile"`
		} `json:"user"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return userID
	}

	name := resp.User.Profile.DisplayName
	if name == "" {
		name = resp.User.Profile.RealName
	}
	if name == "" {
		name = resp.User.Name
	}

	ctx.mu.Lock()
	ctx.userCache[userID] = &slackUserInfo{Name: name}
	ctx.mu.Unlock()

	if name != "" {
		return name
	}
	return userID
}

// CacheUserName 缓存用户名。
func (ctx *SlackMonitorContext) CacheUserName(userID, name string) {
	ctx.mu.Lock()
	ctx.userCache[userID] = &slackUserInfo{Name: name}
	ctx.mu.Unlock()
}

// MarkMessageSeen 消息去重检查。返回 true 表示已见过（应丢弃）。
func (ctx *SlackMonitorContext) MarkMessageSeen(channelID, ts string) bool {
	if channelID == "" || ts == "" {
		return false
	}
	key := channelID + ":" + ts
	now := time.Now().UnixMilli()

	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	if _, seen := ctx.seenMessages[key]; seen {
		return true
	}
	ctx.seenMessages[key] = now

	// 清理过期条目（>60s）
	if len(ctx.seenMessages) > 500 {
		cutoff := now - 60_000
		for k, t := range ctx.seenMessages {
			if t < cutoff {
				delete(ctx.seenMessages, k)
			}
		}
	}

	return false
}

// ShouldDropMismatchedEvent 检查事件的 api_app_id/team_id 是否匹配。
func (ctx *SlackMonitorContext) ShouldDropMismatchedEvent(apiAppID, teamID string) bool {
	if ctx.APIAppID != "" && apiAppID != "" && apiAppID != ctx.APIAppID {
		return true
	}
	if ctx.TeamID != "" && teamID != "" && teamID != ctx.TeamID {
		return true
	}
	return false
}

// IsChannelAllowed 判断频道是否允许接收消息。
func (ctx *SlackMonitorContext) IsChannelAllowed(channelID string, channelType SlackChannelType) bool {
	isDM := channelType == SlackChannelTypeIM
	isMPIM := channelType == SlackChannelTypeMPIM
	isRoom := channelType == SlackChannelTypeChannel || channelType == SlackChannelTypeGroup

	if isDM && !ctx.DMEnabled {
		return false
	}
	if isMPIM && !ctx.GroupDMEnabled {
		return false
	}

	if isRoom && channelID != "" {
		channelName := ctx.ResolveChannelName(channelID)
		chanConfig := ResolveSlackChannelConfig(channelID, channelName, ctx.ChannelConfigs, ctx.RequireMention)
		channelAllowlistConfigured := len(ctx.ChannelConfigs) > 0
		channelAllowed := chanConfig != nil && chanConfig.Allowed
		if !IsSlackChannelAllowedByPolicy(ctx.GroupPolicy, channelAllowlistConfigured, channelAllowed) {
			return false
		}
		// open 策略下只阻止显式拒绝的频道
		hasExplicitConfig := chanConfig != nil && chanConfig.MatchSource != ""
		if !channelAllowed && (ctx.GroupPolicy != "open" || hasExplicitConfig) {
			return false
		}
	}

	return true
}

// InferSlackChannelType 从频道 ID 前缀推断频道类型。
func InferSlackChannelType(channelID string) SlackChannelType {
	trimmed := strings.TrimSpace(channelID)
	if trimmed == "" {
		return SlackChannelTypeChannel
	}
	switch {
	case strings.HasPrefix(trimmed, "D"):
		return SlackChannelTypeIM
	case strings.HasPrefix(trimmed, "G"):
		return SlackChannelTypeGroup
	default:
		return SlackChannelTypeChannel
	}
}

// NormalizeSlackChannelType 规范化频道类型。
func NormalizeSlackChannelType(channelType SlackChannelType, channelID string) SlackChannelType {
	switch channelType {
	case SlackChannelTypeIM, SlackChannelTypeMPIM, SlackChannelTypeChannel, SlackChannelTypeGroup:
		return channelType
	default:
		return InferSlackChannelType(channelID)
	}
}

// ResolveSystemEventSessionKey 为系统事件构建 session key。
func (ctx *SlackMonitorContext) ResolveSystemEventSessionKey(channelID string, channelType SlackChannelType) string {
	if channelID == "" {
		return ctx.MainKey
	}
	normalized := NormalizeSlackChannelType(channelType, channelID)
	isDM := normalized == SlackChannelTypeIM
	isMPIM := normalized == SlackChannelTypeMPIM

	var from string
	switch {
	case isDM:
		from = fmt.Sprintf("slack:%s", channelID)
	case isMPIM:
		from = fmt.Sprintf("slack:group:%s", channelID)
	default:
		from = fmt.Sprintf("slack:channel:%s", channelID)
	}

	// 简化版 resolveSessionKey
	switch ctx.SessionScope {
	case "per-channel", "shared":
		return ctx.MainKey
	default: // "per-sender"
		if ctx.MainKey != "" {
			return ctx.MainKey + ":" + from
		}
		return from
	}
}

// PerformAuthTest 执行 auth.test 获取 bot 信息并缓存。
func (ctx *SlackMonitorContext) PerformAuthTest() {
	resp, err := ctx.Client.AuthTest(context.Background())
	if err != nil {
		log.Printf("[slack:%s] auth.test failed: %v", ctx.AccountID, err)
		return
	}
	ctx.BotUserID = resp.UserID
	ctx.TeamID = resp.TeamID
	ctx.APIAppID = resp.APIAppID
}

// resolveUseAccessGroups 解析 useAccessGroups 配置。
// TS 对照: provider.ts L112 cfg.commands?.useAccessGroups !== false
func resolveUseAccessGroups(cfg *types.OpenAcosmiConfig) bool {
	if cfg == nil || cfg.Commands == nil || cfg.Commands.UseAccessGroups == nil {
		return true // 默认 true
	}
	return *cfg.Commands.UseAccessGroups
}

// RemoveChannelFromCache 从缓存中移除频道。
func (ctx *SlackMonitorContext) RemoveChannelFromCache(channelID string) {
	ctx.mu.Lock()
	delete(ctx.channelCache, channelID)
	ctx.mu.Unlock()
}
