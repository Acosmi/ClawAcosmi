package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// Telegram 消息处理器工厂 — 继承自 src/telegram/bot-message.ts (93L)
// 创建消息处理闭包：buildContext → dispatch

// TelegramMessageProcessor 消息处理函数类型
type TelegramMessageProcessor func(
	msg *TelegramMessage,
	allMedia []TelegramMediaRef,
	storeAllowFrom []string,
	options *TelegramMessageContextOptions,
) error

// TelegramMessageProcessorDeps 消息处理器依赖
type TelegramMessageProcessorDeps struct {
	BotID          int64
	BotUsername    string
	Token          string
	AccountID      string
	AllowFrom      NormalizedAllowFrom
	GroupAllowFrom NormalizedAllowFrom
	TextLimit      int
	StreamMode     string
	ReplyToMode    string
	Config         *types.OpenAcosmiConfig
	TelegramCfg    types.TelegramAccountConfig
	Client         *http.Client
	Deps           *TelegramMonitorDeps

	// DY-012: 配置解析回调 — 对齐 TS bot-message.ts deps
	HistoryLimit               int
	DMPolicy                   string
	AckReactionScope           string
	GroupHistories             map[string][]TelegramHistoryEntry
	ResolveBotTopicsEnabled    func() bool
	ResolveGroupActivation     func(chatID int64, threadID *int, sessionKey string) *bool
	ResolveGroupRequireMention func(chatID int64) bool
	ResolveTelegramGroupConfig func(chatID int64, messageThreadID *int) (*types.TelegramGroupConfig, *types.TelegramTopicConfig)
}

// CreateTelegramMessageProcessor 创建消息处理器。
// 构建完整管线: 消息 → 上下文构建 → agent dispatch → 回复投递。
func CreateTelegramMessageProcessor(deps TelegramMessageProcessorDeps) TelegramMessageProcessor {
	return func(msg *TelegramMessage, allMedia []TelegramMediaRef, storeAllowFrom []string, options *TelegramMessageContextOptions) error {
		// DY-012: 解析群组/话题配置（对齐 TS resolveTelegramGroupConfig）
		var groupConfig *types.TelegramGroupConfig
		var topicConfig *types.TelegramTopicConfig
		if deps.ResolveTelegramGroupConfig != nil {
			groupConfig, topicConfig = deps.ResolveTelegramGroupConfig(msg.Chat.ID, msg.MessageThreadID)
		}

		ctx := BuildTelegramMessageContext(BuildTelegramMessageContextParams{
			Msg:            msg,
			AllMedia:       allMedia,
			StoreAllowFrom: storeAllowFrom,
			Options:        options,
			BotID:          deps.BotID,
			BotUsername:    deps.BotUsername,
			Config:         deps.Config,
			AccountID:      deps.AccountID,
			AllowFrom:      deps.AllowFrom,
			GroupAllowFrom: deps.GroupAllowFrom,
			// DY-012: 传递缺失的配置参数
			HistoryLimit:               deps.HistoryLimit,
			DMPolicy:                   deps.DMPolicy,
			AckReactionScope:           deps.AckReactionScope,
			GroupHistories:             deps.GroupHistories,
			GroupConfig:                groupConfig,
			TopicConfig:                topicConfig,
			Deps:                       deps.Deps,
			ResolveGroupActivation:     deps.ResolveGroupActivation,
			ResolveGroupRequireMention: deps.ResolveGroupRequireMention,
		})
		if ctx == nil {
			return nil
		}

		return DispatchTelegramMessage(context.Background(), DispatchTelegramMessageParams{
			Context:                 ctx,
			Client:                  deps.Client,
			Token:                   deps.Token,
			Config:                  deps.Config,
			ReplyToMode:             deps.ReplyToMode,
			StreamMode:              deps.StreamMode,
			TextLimit:               deps.TextLimit,
			ResolveBotTopicsEnabled: deps.ResolveBotTopicsEnabled,
			Deps:                    deps.Deps,
		})
	}
}

// ---------------------------------------------------------------------------
// DY-012: 配置解析函数 — 对齐 TS bot.ts L263-340
// ---------------------------------------------------------------------------

// ResolveBotTopicsEnabledFunc 创建 resolveBotTopicsEnabled 回调。
// 缓存 getMe 结果以避免重复 API 调用。
// TS 对照: bot.ts L264-288 resolveBotTopicsEnabled
func ResolveBotTopicsEnabledFunc(client *http.Client, token string) func() bool {
	var once sync.Once
	var result bool

	return func() bool {
		once.Do(func() {
			result = false
			if client == nil || token == "" {
				return
			}
			// 直接调用 getMe API 获取 has_topics_enabled
			apiURL := fmt.Sprintf("%s/bot%s/getMe", TelegramAPIBaseURL, token)
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
			if err != nil {
				slog.Debug("telegram: getMe for topics check failed", "err", err)
				return
			}
			resp, err := client.Do(req)
			if err != nil {
				slog.Debug("telegram: getMe for topics check failed", "err", err)
				return
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			var getMeResp struct {
				OK     bool `json:"ok"`
				Result *struct {
					HasTopicsEnabled bool `json:"has_topics_enabled"`
				} `json:"result"`
			}
			if err := json.Unmarshal(body, &getMeResp); err != nil {
				slog.Debug("telegram: getMe decode failed", "err", err)
				return
			}
			if getMeResp.OK && getMeResp.Result != nil {
				result = getMeResp.Result.HasTopicsEnabled
			}
		})
		return result
	}
}

// ResolveGroupActivationFunc 创建 resolveGroupActivation 回调。
// 检查会话存储中的 groupActivation 设置以决定是否需要提及。
// TS 对照: bot.ts L296-320 resolveGroupActivation
// DY-012: 完整实现 — 通过 Deps.LoadSessionEntry 读取 session store。
func ResolveGroupActivationFunc(_ *types.OpenAcosmiConfig, _ string, deps *TelegramMonitorDeps) func(chatID int64, threadID *int, sessionKey string) *bool {
	return func(_ int64, _ *int, sessionKey string) *bool {
		if deps == nil || deps.LoadSessionEntry == nil {
			return nil
		}
		entry, err := deps.LoadSessionEntry(sessionKey)
		if err != nil || entry == nil {
			return nil
		}
		activation := entry["groupActivation"]
		switch activation {
		case "always":
			// "always" → 不需要提及即可激活
			v := false
			return &v
		case "mention":
			// "mention" → 需要提及才能激活
			v := true
			return &v
		default:
			return nil
		}
	}
}

// ResolveGroupRequireMentionFunc 创建 resolveGroupRequireMention 回调。
// 解析群组是否需要提及才能响应。
// TS 对照: bot.ts L321-329 resolveGroupRequireMention
// 注意: requireMention 的群组级/话题级覆盖在 bot_message_context.go 中通过
// groupConfig.RequireMention / topicConfig.RequireMention 处理，此函数仅提供基线默认值。
func ResolveGroupRequireMentionFunc(_ *types.OpenAcosmiConfig, _ string, requireMentionOverride *bool) func(chatID int64) bool {
	return func(_ int64) bool {
		// 默认: 群组中需要提及
		defaultRM := true

		// 检查 opts.requireMention 覆盖（after-config 模式）
		if requireMentionOverride != nil {
			defaultRM = *requireMentionOverride
		}

		return defaultRM
	}
}

// ResolveTelegramGroupConfigFunc 创建 resolveTelegramGroupConfig 回调。
// 从 telegramCfg.groups 中查找群组和话题的配置。
// TS 对照: bot.ts L330-340 resolveTelegramGroupConfig
func ResolveTelegramGroupConfigFunc(telegramCfg types.TelegramAccountConfig) func(chatID int64, messageThreadID *int) (*types.TelegramGroupConfig, *types.TelegramTopicConfig) {
	return func(chatID int64, messageThreadID *int) (*types.TelegramGroupConfig, *types.TelegramTopicConfig) {
		groups := telegramCfg.Groups
		if len(groups) == 0 {
			return nil, nil
		}

		groupKey := strconv.FormatInt(chatID, 10)

		// 查找精确匹配或通配符
		groupConfig, ok := groups[groupKey]
		if !ok {
			groupConfig = groups["*"]
		}
		if groupConfig == nil {
			return nil, nil
		}

		// 查找话题配置
		var topicConfig *types.TelegramTopicConfig
		if messageThreadID != nil && groupConfig.Topics != nil {
			topicKey := strconv.Itoa(*messageThreadID)
			topicConfig = groupConfig.Topics[topicKey]
		}

		return groupConfig, topicConfig
	}
}

// isTruthyString 检查字符串是否为 truthy 值
func isTruthyString(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "1" || s == "true" || s == "yes" || s == "on"
}

// formatErrorForLog 格式化错误消息用于日志
func formatErrorForLog(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%v", err)
}
