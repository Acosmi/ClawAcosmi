package reply

import (
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// TS 对照: auto-reply/reply/queue/settings.ts (69L)
// 设置解析：多级优先级链解析队列配置。

// PluginDebounceProvider 获取频道插件级 debounce 默认值（DI 注入）。
// 由 gateway 启动时注入 channels.GetPluginDebounce。
// 返回 nil 表示该频道无插件默认值。
var PluginDebounceProvider func(channelKey string) *int

// ResolveQueueSettingsParams 队列设置解析参数。
type ResolveQueueSettingsParams struct {
	Cfg           *types.OpenAcosmiConfig
	Channel       string
	SessionEntry  *SessionEntry
	InlineMode    QueueMode
	InlineOptions *QueueOptions
}

// defaultQueueModeForChannel 获取频道的默认队列模式。
func defaultQueueModeForChannel(_ string) QueueMode {
	return QueueModeCollect
}

// resolveChannelDebounce 解析频道级 debounce 覆盖。
func resolveChannelDebounce(byChannel types.InboundDebounceByProvider, channelKey string) *int {
	if channelKey == "" || byChannel == nil {
		return nil
	}
	value, ok := byChannel[channelKey]
	if !ok {
		return nil
	}
	if value < 0 {
		value = 0
	}
	return &value
}

// resolveByChannelMode 从 QueueModeByProvider 解析频道级队列模式。
func resolveByChannelMode(byChannel *types.QueueModeByProvider, channelKey string) string {
	if byChannel == nil || channelKey == "" {
		return ""
	}
	switch channelKey {
	case "whatsapp":
		return string(byChannel.WhatsApp)
	case "telegram":
		return string(byChannel.Telegram)
	case "discord":
		return string(byChannel.Discord)
	case "googlechat":
		return string(byChannel.GoogleChat)
	case "slack":
		return string(byChannel.Slack)
	case "signal":
		return string(byChannel.Signal)
	case "imessage":
		return string(byChannel.IMessage)
	case "msteams":
		return string(byChannel.MSTeams)
	case "webchat":
		return string(byChannel.WebChat)
	default:
		return ""
	}
}

// ResolveQueueSettings 通过多级优先级链解析队列设置。
// 优先级: inline → session → channel config → global config → defaults
func ResolveQueueSettings(params ResolveQueueSettingsParams) QueueSettings {
	channelKey := strings.TrimSpace(strings.ToLower(params.Channel))

	var queueCfg *types.QueueConfig
	if params.Cfg != nil && params.Cfg.Messages != nil {
		queueCfg = params.Cfg.Messages.Queue
	}

	// 1. 解析队列模式
	var providerModeRaw string
	if queueCfg != nil {
		providerModeRaw = resolveByChannelMode(queueCfg.ByChannel, channelKey)
	}

	resolvedMode := params.InlineMode
	if resolvedMode == "" && params.SessionEntry != nil {
		resolvedMode = NormalizeQueueMode(params.SessionEntry.QueueMode)
	}
	if resolvedMode == "" {
		resolvedMode = NormalizeQueueMode(providerModeRaw)
	}
	if resolvedMode == "" && queueCfg != nil {
		resolvedMode = NormalizeQueueMode(string(queueCfg.Mode))
	}
	if resolvedMode == "" {
		resolvedMode = defaultQueueModeForChannel(channelKey)
	}

	// 2. 解析 debounce
	var debounceMs *int
	if params.InlineOptions != nil && params.InlineOptions.DebounceMs != nil {
		debounceMs = params.InlineOptions.DebounceMs
	}
	if debounceMs == nil && params.SessionEntry != nil && params.SessionEntry.QueueDebounceMs > 0 {
		v := params.SessionEntry.QueueDebounceMs
		debounceMs = &v
	}
	if debounceMs == nil && queueCfg != nil {
		debounceMs = resolveChannelDebounce(queueCfg.DebounceMsByChannel, channelKey)
	}
	// resolvePluginDebounce: 对齐 TS settings.ts resolvePluginDebounce
	if debounceMs == nil && channelKey != "" && PluginDebounceProvider != nil {
		debounceMs = PluginDebounceProvider(channelKey)
	}
	if debounceMs == nil && queueCfg != nil {
		debounceMs = queueCfg.DebounceMs
	}
	if debounceMs == nil {
		v := DefaultQueueDebounceMs
		debounceMs = &v
	}
	// clamp >= 0
	if debounceMs != nil && *debounceMs < 0 {
		v := 0
		debounceMs = &v
	}

	// 3. 解析 cap
	var capVal *int
	if params.InlineOptions != nil && params.InlineOptions.Cap != nil {
		capVal = params.InlineOptions.Cap
	}
	if capVal == nil && params.SessionEntry != nil && params.SessionEntry.QueueCap > 0 {
		v := params.SessionEntry.QueueCap
		capVal = &v
	}
	if capVal == nil && queueCfg != nil {
		capVal = queueCfg.Cap
	}
	if capVal == nil {
		v := DefaultQueueCap
		capVal = &v
	}
	// clamp >= 1
	if capVal != nil && *capVal < 1 {
		v := 1
		capVal = &v
	}

	// 4. 解析 drop policy
	var dropPolicy QueueDropPolicy
	if params.InlineOptions != nil && params.InlineOptions.DropPolicy != "" {
		dropPolicy = params.InlineOptions.DropPolicy
	}
	if dropPolicy == "" && params.SessionEntry != nil && params.SessionEntry.QueueDrop != "" {
		dropPolicy = NormalizeQueueDropPolicy(params.SessionEntry.QueueDrop)
	}
	if dropPolicy == "" && queueCfg != nil {
		dropPolicy = NormalizeQueueDropPolicy(string(queueCfg.Drop))
	}
	if dropPolicy == "" {
		dropPolicy = DefaultQueueDrop
	}

	return QueueSettings{
		Mode:       resolvedMode,
		DebounceMs: debounceMs,
		Cap:        capVal,
		DropPolicy: dropPolicy,
	}
}
